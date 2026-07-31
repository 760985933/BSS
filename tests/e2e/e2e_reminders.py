#!/usr/bin/env python3
# Sprint 6 端到端验证：全链路造数 → 提醒扫描 → 通知/仪表盘
import json
import urllib.request
import urllib.error
import sys

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
API = BASE + "/api/v1"
passed = 0
failed = 0


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
        return resp.status, (json.loads(b)["data"] if b else {})
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            j = json.loads(raw)
            return e.code, (j.get("data", {}) if isinstance(j, dict) else {})
        except Exception:
            return e.code, {}


def check(name, cond, extra=""):
    global passed, failed
    if cond:
        passed += 1
        print("PASS", name)
    else:
        failed += 1
        print("FAIL", name, extra)


# 1) 登录 admin
st, body = req("POST", "/auth/login", {"email": "admin@bss.local", "password": "admin123"})
check("admin 登录 200", st == 200 and "token" in body)
token = body["token"]

# 采集仪表盘基线（建数前），后续断言仅校验本脚本产生的增量，隔离 seed/其他脚本副作用
_, base_dash = req("GET", "/dashboard", token=token)
_bc = base_dash.get("cards", {}) if isinstance(base_dash, dict) else {}
base_signed = _bc.get("signed_this_month_cent", 0) or 0
base_paid = _bc.get("paid_this_month_cent", 0) or 0
base_overdue = _bc.get("overdue_amount_cent", 0) or 0

# 2) 客户
st, c = req("POST", "/customers", {"code": "KH-E2E6", "name": "e2e6客户", "industry": "it", "source": "web", "level": "a"}, token)
check("建客户 200", st == 200)
cid = c.get("id")

# 3) 进行中商单（prospecting，撑 open_deals）
st, d1 = req("POST", "/deals", {"customer_id": cid, "title": "进行中商机", "amount_cent": 40000, "probability": 10}, token)
check("建进行中商单 200", st == 200)

# 4) 赢单商单（走完整流转）
st, d2 = req("POST", "/deals", {"customer_id": cid, "title": "赢单商机", "amount_cent": 120000, "probability": 100}, token)
check("建赢单草稿 200", st == 200)
did = d2.get("id")
for to in ["qualifying", "proposal", "negotiating"]:
    req("POST", "/deals/%s/status" % did, {"to": to}, token)
# 经审批流到达 won（普通 ChangeStatus 不允许 negotiating→won）
st, ap = req("POST", "/approvals", {"entity_type": "deal", "entity_id": did, "kind": "deal_discount", "amount_cent": 0}, token)
if st == 200:
    req("POST", "/approvals/%s/approve" % ap.get("id"), token=token)
st, d2w = req("GET", "/deals/%s" % did, token=token)
check("商单流转至 won 200", d2w.get("status") == "won", d2w.get("status"))

# 5) 签约合同（到期日 20 天后）
st, ct = req("POST", "/contracts", {"customer_id": cid, "title": "e2e6合同", "amount_cent": 200000,
                                    "sign_date": "2026-07-30", "start_date": "2026-07-30", "expire_date": "2026-08-19"}, token)
check("建合同 200", st == 200)
ctid = ct.get("id")
req("POST", "/contracts/%s/status" % ctid, {"to": "pending"}, token)
# signed 须经合同签约审批流
st, ap = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
if st == 200:
    st, _ = req("POST", "/approvals/%s/approve" % ap.get("id"), token=token)
st, ct_s = req("GET", "/contracts/%s" % ctid, token=token)
check("合同→signed 200", st == 200 and ct_s.get("status") == "signed", ct_s.get("status"))

# 6) 逾期回款计划 + 部分到账
st, p = req("POST", "/contracts/%s/plans" % ctid, {"period_no": 1, "due_date": "2026-07-10", "amount_cent": 80000}, token)
check("建逾期计划 200", st == 200)
pid = p.get("id")
st, _ = req("POST", "/contracts/%s/records" % ctid, {"records": [{"plan_id": pid, "amount_cent": 30000, "paid_at": "2026-07-30", "method": "bank"}]}, token)
check("部分到账 200（计划转 partial）", st == 200)

# 7) 手动触发提醒扫描
st, sc = req("POST", "/admin/scan-reminders", token=token)
check("扫描提醒 200", st == 200)
created = sc.get("created", 0)
check("生成 >=2 条提醒（合同到期 + 回款逾期）", created >= 2, "created=%s" % created)

# 8) 重复扫描应去重（created=0）
st, sc2 = req("POST", "/admin/scan-reminders", token=token)
check("重复扫描去重 created=0", st == 200 and sc2.get("created", -1) == 0, "created=%s" % sc2.get("created"))

# 9) 通知列表 + 未读
st, nt = req("GET", "/notifications", token=token)
check("通知列表 200", st == 200)
items = nt.get("items", [])
types = set(n.get("type") for n in items)
check("含 contract_expiring 通知", "contract_expiring" in types)
check("含 payment_overdue 通知", "payment_overdue" in types)

st, uc = req("GET", "/notifications/unread-count", token=token)
check("未读角标 >=2", st == 200 and uc.get("count", 0) >= 2, "count=%s" % uc.get("count"))

# 10) 仪表盘聚合
st, dash = req("GET", "/dashboard", token=token)
check("仪表盘 200", st == 200)
cards = dash.get("cards", {})
check("本月签约金额=基线+200000", cards.get("signed_this_month_cent") == base_signed + 200000, str(cards))
check("本月回款金额=基线+30000", cards.get("paid_this_month_cent") == base_paid + 30000, str(cards))
check("进行中商单>=1", cards.get("open_deals", 0) >= 1, str(cards))
check("逾期金额=基线+50000（80000-30000）", cards.get("overdue_amount_cent") == base_overdue + 50000, str(cards))
check("即将到期合同列表>=1", len(dash.get("expiring_contracts", [])) >= 1)
check("逾期回款列表>=1", len(dash.get("overdue_plans", [])) >= 1)
check("近期赢单列表>=1", len(dash.get("recent_won_deals", [])) >= 1)

# 11) 标记单条已读
nid = items[0]["id"]
before = uc.get("count", 0)
st, _ = req("POST", "/notifications/%s/read" % nid, token=token)
check("标记已读 200", st == 200)
st, uc2 = req("GET", "/notifications/unread-count", token=token)
check("未读数减少", st == 200 and uc2.get("count", 0) == before - 1, "before=%s after=%s" % (before, uc2.get("count")))

# 12) 全部已读
st, _ = req("POST", "/notifications/read-all", token=token)
check("全部已读 200", st == 200)
st, uc3 = req("GET", "/notifications/unread-count", token=token)
check("未读数归零", st == 200 and uc3.get("count", -1) == 0, "count=%s" % uc3.get("count"))

print("\nRESULT:", "ALL PASS" if failed == 0 else "HAS FAIL", "| passed=%d failed=%d" % (passed, failed))
sys.exit(1 if failed else 0)
