#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Sprint 5 回款端到端验证：覆盖计划总额上限/已核销锁定/跨计划核销/财务角色/汇总。"""
import json
import urllib.request
import urllib.error
import sys

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
API = BASE + "/api/v1"

passed = 0
failed = 0


def req(method, path, body=None, token=None, raw=False):
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
        resp = urllib.request.urlopen(r)
        b = resp.read()
    except urllib.error.HTTPError as e:
        return e.code, (json.loads(e.read()) if e.fp else {})
    j = json.loads(b) if b else {}
    if raw:
        return resp.status, j
    return resp.status, (j.get("data", j))


def check(name, cond, extra=""):
    global passed, failed
    if cond:
        passed += 1
        print("PASS", name)
    else:
        failed += 1
        print("FAIL", name, extra)


def login(email, password):
    st, b = req("POST", "/auth/login", {"email": email, "password": password})
    if st != 200 or "token" not in b:
        return None, b
    return b["token"], b


print("=== 登录 admin ===")
token, _ = login("admin@bss.local", "admin123")
check("admin 登录 200", token is not None)
if not token:
    print("E2E_EXIT:1"); raise SystemExit(1)

print("=== 建客户 + 合同(signed, 额 100000) ===")
st, c = req("POST", "/customers", {"code": "KH-PY1", "name": "回款客户", "industry": "it", "source": "web", "level": "a"}, token)
check("建客户 200", st == 200, str(st))
cid = c.get("id")
st, ct = req("POST", "/contracts", {"customer_id": cid, "title": "回款合同", "amount_cent": 100000}, token)
check("建合同 200", st == 200, str(st))
ctid = ct.get("id")
st, _ = req("POST", "/contracts/%s/status" % ctid, {"to": "pending"}, token)
check("合同推进 pending 200", st == 200, str(st))
# signed 须经合同签约审批流（普通状态推进不允许 directly→signed）
st, ap = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
if st == 200:
    st, _ = req("POST", "/approvals/%s/approve" % ap.get("id"), token=token)
st, ct_s = req("GET", "/contracts/%s" % ctid, token=token)
check("合同推进 signed 200", st == 200 and ct_s.get("status") == "signed", ct_s.get("status"))

print("=== 计划总额上限 ===")
st, p1 = req("POST", "/contracts/%s/plans" % ctid, {"period_no": 1, "due_date": "2020-01-01", "amount_cent": 60000}, token)
check("计划1(60000) 200", st == 200, str(st))
st, p2 = req("POST", "/contracts/%s/plans" % ctid, {"period_no": 2, "due_date": "2020-02-01", "amount_cent": 40000}, token)
check("计划2(40000) 200 总额=100000", st == 200, str(st))
st, _ = req("POST", "/contracts/%s/plans" % ctid, {"period_no": 3, "due_date": "2020-03-01", "amount_cent": 1}, token)
check("计划3 超合同额→422", st == 422, str(st))
pid1 = p1.get("id")

print("=== 已核销计划锁定（改/删 422）===")
st, _ = req("POST", "/contracts/%s/records" % ctid, {"records": [{"plan_id": pid1, "amount_cent": 60000, "paid_at": "2020-01-10", "method": "bank"}]}, token)
check("核销计划1(60000) 200", st == 200, str(st))
st, _ = req("PUT", "/contracts/%s/plans/%s" % (ctid, pid1), {"period_no": 1, "due_date": "2020-01-01", "amount_cent": 50000}, token)
check("已核销计划编辑→422", st == 422, str(st))
st, _ = req("DELETE", "/contracts/%s/plans/%s" % (ctid, pid1), token=token)
check("已核销计划删除→422", st == 422, str(st))

print("=== 汇总准确到分 ===")
st, sum1 = req("GET", "/contracts/%s/payment-summary" % ctid, token=token)
check("汇总 200", st == 200, str(st))
check("应收=100000", sum1.get("receivable_cent") == 100000, str(sum1))
check("已收=60000", sum1.get("received_cent") == 60000, str(sum1))
check("余额=40000", sum1.get("balance_cent") == 40000, str(sum1))
check("逾期=40000(计划2未核销且逾期)", sum1.get("overdue_cent") == 40000, str(sum1))

print("=== 跨合同核销拒绝 422 ===")
st, c2 = req("POST", "/customers", {"code": "KH-PY2", "name": "回款客户B", "industry": "it", "source": "web", "level": "a"}, token)
cid2 = c2.get("id")
st, ct2 = req("POST", "/contracts", {"customer_id": cid2, "title": "合同B", "amount_cent": 50000}, token)
ctid2 = ct2.get("id")
req("POST", "/contracts/%s/status" % ctid2, {"to": "signed"}, token)
st, px = req("POST", "/contracts/%s/plans" % ctid2, {"period_no": 1, "due_date": "2020-05-01", "amount_cent": 50000}, token)
st, _ = req("POST", "/contracts/%s/records" % ctid, {"records": [{"plan_id": px.get("id"), "amount_cent": 1000, "paid_at": "2020-05-02"}]}, token)
check("跨合同核销→422", st == 422, str(st))

print("=== 角色：销售禁止录入(403)，财务允许(200) ===")
st, emp_sales = req("POST", "/employees", {"name": "销售A", "email": "sales_a@bss.local", "phone": "1", "dept": "s", "position": "x", "role": "sales"}, token)
check("建销售员工 200", st == 200, str(st))
sales_pwd = emp_sales.get("initial_password")
st, emp_fin = req("POST", "/employees", {"name": "财务A", "email": "fin_a@bss.local", "phone": "1", "dept": "f", "position": "x", "role": "finance"}, token)
check("建财务员工 200", st == 200, str(st))
fin_pwd = emp_fin.get("initial_password")

sales_token, _ = login("sales_a@bss.local", sales_pwd)
fin_token, _ = login("fin_a@bss.local", fin_pwd)
check("销售登录 200", sales_token is not None)
check("财务登录 200", fin_token is not None)

pid2 = p2.get("id")
st, _ = req("POST", "/contracts/%s/records" % ctid, {"records": [{"plan_id": pid2, "amount_cent": 40000, "paid_at": "2020-02-10", "method": "bank"}]}, token=sales_token)
check("销售录入回款→403", st == 403, str(st))
st, _ = req("POST", "/contracts/%s/records" % ctid, {"records": [{"plan_id": pid2, "amount_cent": 40000, "paid_at": "2020-02-10", "method": "bank"}]}, token=fin_token)
check("财务录入回款→200", st == 200, str(st))

st, sum2 = req("GET", "/contracts/%s/payment-summary" % ctid, token=token)
check("财务核销后 已收=100000", sum2.get("received_cent") == 100000, str(sum2))
check("财务核销后 余额=0", sum2.get("balance_cent") == 0, str(sum2))
check("财务核销后 逾期=0", sum2.get("overdue_cent") == 0, str(sum2))

print("\n=== RESULT: %d PASS / %d FAIL ===" % (passed, failed))
raise SystemExit(1 if failed else 0)
