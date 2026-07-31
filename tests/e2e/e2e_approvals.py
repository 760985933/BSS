#!/usr/bin/env python3
# M2-1 审批流端到端验证
import json
import urllib.request
import urllib.error
import sys

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
API = BASE + "/api/v1"
PASS = 0
FAIL = 0


def req(method, path, body=None, token=None):
    url = API + path
    headers = {}
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        b = resp.read()
        return resp.status, (json.loads(b) if b else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, {}


def check(name, cond, detail=""):
    global PASS, FAIL
    if cond:
        PASS += 1
        print("PASS", name)
    else:
        FAIL += 1
        print("FAIL", name, detail)


def login(email, pwd="admin123"):
    st, b = req("POST", "/auth/login", {"email": email, "password": pwd})
    if st == 200 and "token" in b.get("data", {}):
        return b["data"]["token"]
    return None


print("=== login admin ===")
token = login("admin@bss.local")
check("admin login", token is not None)
if not token:
    print("EXIT early"); raise SystemExit(1)

# 建客户
st, c = req("POST", "/customers", {"code": "KH-AP1", "name": "审批客户", "industry": "it", "source": "web", "level": "a"}, token)
check("建客户 200", st == 200, str(st))
cid = c.get("data", {}).get("id")

# 建商单并推进到 negotiating
st, d = req("POST", "/deals", {"customer_id": cid, "title": "折扣商单", "amount_cent": 100000}, token)
check("建商单 200", st == 200, str(st))
did = d.get("data", {}).get("id")
for to in ["qualifying", "proposal", "negotiating"]:
    st, _ = req("POST", "/deals/%s/status" % did, {"to": to}, token)
    check("商单→%s 200" % to, st == 200, str(st))

# 提交商单折扣审批
st, ap = req("POST", "/approvals", {"entity_type": "deal", "entity_id": did, "kind": "deal_discount", "amount_cent": 10000}, token)
check("提交商单折扣审批 200", st == 200, str(st))
apid = ap.get("data", {}).get("id")
check("审批单状态 pending", ap.get("data", {}).get("status") == "pending")
# 商单应进入 pending_approval
st, d2 = req("GET", "/deals/%s" % did, token=token)
check("商单进入 pending_approval", d2.get("data", {}).get("status") == "pending_approval", d2.get("data", {}).get("status"))

# 审批通过 → 赢单 + 折扣落地
st, _ = req("POST", "/approvals/%s/approve" % apid, token=token)
check("审批通过 200", st == 200, str(st))
st, d3 = req("GET", "/deals/%s" % did, token=token)
check("商单审批后 won", d3.get("data", {}).get("status") == "won", d3.get("data", {}).get("status"))
check("折扣金额落地 10000", d3.get("data", {}).get("discount_amount_cent") == 10000, str(d3.get("data", {}).get("discount_amount_cent")))

# 合同签约审批：建合同 → pending → 提交 → 通过 → signed
st, ct = req("POST", "/contracts", {"customer_id": cid, "title": "签约合同", "amount_cent": 50000}, token)
check("建合同 200", st == 200, str(st))
ctid = ct.get("data", {}).get("id")
st, _ = req("POST", "/contracts/%s/status" % ctid, {"to": "pending"}, token)
check("合同→pending 200", st == 200, str(st))
st, ap2 = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
check("提交合同签约审批 200", st == 200, str(st))
ap2id = ap2.get("data", {}).get("id")
st, ct2 = req("GET", "/contracts/%s" % ctid, token=token)
check("合同进入 pending_approval", ct2.get("data", {}).get("status") == "pending_approval", ct2.get("data", {}).get("status"))
st, _ = req("POST", "/approvals/%s/approve" % ap2id, token=token)
check("合同审批通过 200", st == 200, str(st))
st, ct3 = req("GET", "/contracts/%s" % ctid, token=token)
check("合同审批后 signed", ct3.get("data", {}).get("status") == "signed", ct3.get("data", {}).get("status"))

# 驳回路径：另一合同 → pending → 提交 → 驳回 → 回退 pending
st, ct4 = req("POST", "/contracts", {"customer_id": cid, "title": "驳回合同", "amount_cent": 50000}, token)
ct4id = ct4.get("data", {}).get("id")
req("POST", "/contracts/%s/status" % ct4id, {"to": "pending"}, token=token)
st, ap3 = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ct4id, "kind": "contract_sign"}, token)
ap3id = ap3.get("data", {}).get("id")
st, _ = req("POST", "/approvals/%s/reject" % ap3id, {"reason": "风险过高"}, token=token)
check("驳回 200", st == 200, str(st))
st, ct5 = req("GET", "/contracts/%s" % ct4id, token=token)
check("驳回后回退 pending", ct5.get("data", {}).get("status") == "pending", ct5.get("data", {}).get("status"))

# 非法：draft 合同直接提交签约审批 → 422
st, _ = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
check("draft 合同提交审批 422", st == 422, str(st))

# 角色门控：建销售账号，登录后销售可提交、不可审批
st, e = req("POST", "/employees", {"name": "审批销售", "email": "apr-sales@bss.local", "phone": "13900000000",
                                    "dept": "销售", "position": "销售", "role": "sales"}, token)
check("建销售账号 200", st == 200, str(st))
ep = e.get("data", {}).get("initial_password")
stok = login("apr-sales@bss.local", ep)
check("销售登录 200", stok is not None)
# 销售提交审批：建合同→pending→提交
st, ct6 = req("POST", "/contracts", {"customer_id": cid, "title": "销售提交合同", "amount_cent": 50000}, stok)
ct6id = ct6.get("data", {}).get("id")
req("POST", "/contracts/%s/status" % ct6id, {"to": "pending"}, token=stok)
st, ap4 = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ct6id, "kind": "contract_sign"}, stok)
check("销售可提交审批 200", st == 200, str(st))
ap4id = ap4.get("data", {}).get("id")
# 销售审批 → 403
st, _ = req("POST", "/approvals/%s/approve" % ap4id, token=stok)
check("销售不可审批 403", st == 403, str(st))
# admin 审批 → 200
st, _ = req("POST", "/approvals/%s/approve" % ap4id, token=token)
check("admin 可审批 200", st == 200, str(st))

# 列表
st, lst = req("GET", "/approvals", token=token)
check("审批列表可拉取", st == 200, str(st))
check("审批列表非空", lst.get("data", {}).get("total", 0) >= 4, str(lst.get("data", {}).get("total")))

print("\nRESULT:", "ALL PASS" if FAIL == 0 else "HAS FAIL", "PASS=%d FAIL=%d" % (PASS, FAIL))
