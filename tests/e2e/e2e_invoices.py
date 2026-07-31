#!/usr/bin/env python3
# M2-2 开票管理端到端验证
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
    raise SystemExit(1)

# 客户 + 合同 → 签约（走审批）
st, c = req("POST", "/customers", {"code": "KH-INV1", "name": "开票客户", "industry": "it", "source": "web", "level": "a"}, token)
cid = c.get("data", {}).get("id")
st, ct = req("POST", "/contracts", {"customer_id": cid, "title": "开票合同", "amount_cent": 100000}, token)
ctid = ct.get("data", {}).get("id")
req("POST", "/contracts/%s/status" % ctid, {"to": "pending"}, token)
st, ap = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
req("POST", "/approvals/%s/approve" % ap.get("data", {}).get("id"), token=token)
st, ct2 = req("GET", "/contracts/%s" % ctid, token=token)
check("合同已签约 signed", ct2.get("data", {}).get("status") == "signed", ct2.get("data", {}).get("status"))

# 新建待开发票 30000
st, inv = req("POST", "/invoices", {"contract_id": ctid, "amount_cent": 30000, "remark": "首期"}, token)
check("新建发票 200", st == 200, str(st))
check("发票状态 draft", inv.get("data", {}).get("status") == "draft")
invid = inv.get("data", {}).get("id")

# 开票
st, _ = req("POST", "/invoices/%s/issue" % invid, token=token)
check("开票 200", st == 200, str(st))
st, inv2 = req("GET", "/invoices/%s" % invid, token=token)
check("发票状态 issued", inv2.get("data", {}).get("status") == "issued", inv2.get("data", {}).get("status"))
check("开票日期已填", inv2.get("data", {}).get("issued_at") != "", inv2.get("data", {}).get("issued_at"))

# 超额：已开 30000 + 80000 = 110000 > 100000 → 422
st, _ = req("POST", "/invoices", {"contract_id": ctid, "amount_cent": 80000}, token)
check("超额开票 422", st == 422, str(st))

# 边界：30000 + 70000 = 100000 恰好 = 合同额 → 允许
st, inv3 = req("POST", "/invoices", {"contract_id": ctid, "amount_cent": 70000}, token)
check("恰好等于合同额 200", st == 200, str(st))
inv3id = inv3.get("data", {}).get("id")

# 作废
st, _ = req("POST", "/invoices/%s/void" % inv3id, token=token)
check("作废 200", st == 200, str(st))
st, inv3v = req("GET", "/invoices/%s" % inv3id, token=token)
check("发票状态 voided", inv3v.get("data", {}).get("status") == "voided", inv3v.get("data", {}).get("status"))

# 删除待开发票（先新建一张 draft）
st, inv4 = req("POST", "/invoices", {"contract_id": ctid, "amount_cent": 1000}, token)
inv4id = inv4.get("data", {}).get("id")
st, _ = req("DELETE", "/invoices/%s" % inv4id, token=token)
check("删除待开发票 200", st == 200, str(st))

# 角色门控：销售不可新建发票
st, e = req("POST", "/employees", {"name": "开票销售", "email": "inv-sales@bss.local", "phone": "13900000001",
                                   "dept": "销售", "position": "销售", "role": "sales"}, token)
ep = e.get("data", {}).get("initial_password")
stok = login("inv-sales@bss.local", ep)
check("销售登录 200", stok is not None)
st, _ = req("POST", "/invoices", {"contract_id": ctid, "amount_cent": 1000}, stok)
check("销售新建发票 403", st == 403, str(st))

print("\nRESULT:", "ALL PASS" if FAIL == 0 else "HAS FAIL", "PASS=%d FAIL=%d" % (PASS, FAIL))
