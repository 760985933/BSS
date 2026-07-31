#!/usr/bin/env python3
# 报表中心 E2E（M2-3）：趋势/排行/漏斗接口 + CSV 导出
import sys, json, urllib.request, urllib.error

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
ADMIN = ("admin@bss.local", "admin123")
SALES = ("demo-sales@bss.local", "Bss@1234")

passed = 0
failed = 0

def check(name, cond, extra=""):
    global passed, failed
    if cond:
        passed += 1
        print(f"  PASS  {name}")
    else:
        failed += 1
        print(f"  FAIL  {name}  {extra}")

def req(method, path, token=None, raw=False, params=None):
    url = BASE + "/api/v1" + path
    if params:
        url += ("&" if "?" in url else "?") + params
    r = urllib.request.Request(url, method=method)
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        body = resp.read()
        if raw:
            return resp, body
        return resp, json.loads(body)
    except urllib.error.HTTPError as e:
        return e, (e.read().decode() if not raw else e.read())

def login(email, pwd):
    url = BASE + "/api/v1/auth/login"
    body = json.dumps({"email": email, "password": pwd}).encode()
    r = urllib.request.Request(url, data=body, method="POST")
    r.add_header("Content-Type", "application/json")
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        d = json.loads(resp.read())
        return d.get("data", {}).get("token")
    except Exception:
        return None

def get_json(path, token, params=None):
    resp, d = req("GET", path, token, params=params)
    return resp, d

print("== 登录 ==")
at = login(*ADMIN)
check("admin 登录获取 token", bool(at))
st = login(*SALES)
check("销售登录获取 token", bool(st))

print("== 月度签约趋势 ==")
resp, d = get_json("/reports/sign-trend", at, "months=12")
check("sign-trend 200", resp.status == 200, str(resp.status))
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("rows", [])
check("sign-trend 返回 12 个月", len(rows) == 12, f"len={len(rows)}")
check("sign-trend 行含 month/amount_cent", all("month" in r and "amount_cent" in r for r in rows))
this_month = __import__("time").strftime("%Y-%m")
cur = next((r["amount_cent"] for r in rows if r["month"] == this_month), None)
check("当月签约额 > 0（种子合同今日签约）", isinstance(cur, int) and cur > 0, f"cur={cur}")

print("== 月度回款趋势 ==")
resp, d = get_json("/reports/payment-trend", at, "months=12")
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("rows", [])
check("payment-trend 200", resp.status == 200, str(resp.status))
check("payment-trend 返回 12 个月", len(rows) == 12, f"len={len(rows)}")
check("payment-trend 行含 month/amount_cent", all("month" in r and "amount_cent" in r for r in rows))

print("== 销售排行 ==")
resp, d = get_json("/reports/sales-rank", at)
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("rows", [])
check("sales-rank 200", resp.status == 200, str(resp.status))
check("sales-rank 至少 1 名销售", len(rows) >= 1, f"len={len(rows)}")
check("sales-rank 行含 owner_name/signed_cent", all("owner_name" in r and "signed_cent" in r for r in rows))
check("销售排行按签约额降序", all(rows[i]["signed_cent"] >= rows[i+1]["signed_cent"] for i in range(len(rows)-1)))

print("== 客户转化漏斗 ==")
resp, d = get_json("/reports/funnel", at)
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("rows", [])
check("funnel 200", resp.status == 200, str(resp.status))
check("funnel 5 个阶段", len(rows) == 5, f"len={len(rows)}")
stages = {r["stage"] for r in rows}
check("funnel 含 won 阶段", "won" in stages, str(stages))
won = next((r for r in rows if r["stage"] == "won"), None)
check("won 阶段赢单数 > 0", won and won["count"] > 0, str(won))

print("== CSV 导出（各类型）==")
for typ in ["sign_trend", "payment_trend", "sales_rank", "funnel"]:
    resp, body = req("GET", "/reports/export", at, raw=True, params=f"type={typ}")
    ct = resp.headers.get("Content-Type", "") if hasattr(resp, "headers") else ""
    check(f"export {typ} 200", resp.status == 200, str(resp.status))
    check(f"export {typ} text/csv", "text/csv" in ct, ct)
    txt = body.decode("utf-8", "replace") if isinstance(body, bytes) else body
    check(f"export {typ} 含 BOM", txt.startswith("﻿"))
    check(f"export {typ} 含表头", "," in txt.splitlines()[0] if txt.splitlines() else False)

print("== 数据范围：销售仅见本人 ==")
resp, d = get_json("/reports/sales-rank", st)
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("rows", [])
check("销售排行仅含本人(演示销售)", all(r["owner_name"] == "演示销售" for r in rows), str([r["owner_name"] for r in rows]))

print(f"\n结果: {passed} 通过 / {failed} 失败")
sys.exit(1 if failed else 0)
