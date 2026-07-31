#!/usr/bin/env python3
# 审计查询 + 离职交接 E2E（M2-4）：audit-logs 接口 + 离职交接转移流程
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

def req(method, path, token=None, raw=False, params=None, body=None, ctype="application/json"):
    url = BASE + "/api/v1" + path
    if params:
        url += ("&" if "?" in url else "?") + params
    r = urllib.request.Request(url, method=method, data=body)
    if token:
        r.add_header("Authorization", "Bearer " + token)
    if body is not None and ctype:
        r.add_header("Content-Type", ctype)
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        b = resp.read()
        if raw:
            return resp, b
        return resp, json.loads(b)
    except urllib.error.HTTPError as e:
        return e, (e.read().decode() if not raw else e.read())

def login(email, pwd):
    url = BASE + "/api/v1/auth/login"
    body = json.dumps({"email": email, "password": pwd}).encode()
    r = urllib.request.Request(url, data=body, method="POST")
    r.add_header("Content-Type", "application/json")
    try:
        resp = urllib.request.urlopen(r, timeout=10)
        return json.loads(resp.read()).get("data", {}).get("token")
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

print("== 审计查询：权限 ==")
resp, d = get_json("/audit-logs", at)
data = d.get("data", {}) if isinstance(d, dict) else {}
check("admin 可访问 audit-logs (200)", resp.status == 200, str(resp.status))
rows = data.get("list", [])
check("audit-logs 返回列表非空", isinstance(rows, list) and len(rows) > 0, f"len={len(rows)}")
check("audit-logs 含 total 字段", isinstance(data.get("total"), int) and data["total"] > 0, str(data.get("total")))
if rows:
    sample = rows[0]
    check("审计行含 entity_type/action/operator_id", all(k in sample for k in ("entity_type", "action", "operator_id")), str(list(sample.keys())))

resp, d = get_json("/audit-logs", st)
check("销售被禁止访问 audit-logs (403)", resp.status == 403, str(resp.status))

print("== 审计查询：过滤 ==")
resp, d = get_json("/audit-logs", at, "action=create")
data = d.get("data", {}) if isinstance(d, dict) else {}
rows = data.get("list", [])
check("按 action=create 过滤 200", resp.status == 200, str(resp.status))
check("action=create 结果均为 create", all(r.get("action") == "create" for r in rows), str([r.get("action") for r in rows]))
check("action=create 总数 >= 过滤结果", data.get("total", 0) >= len(rows), f"total={data.get('total')} len={len(rows)}")

resp, d = get_json("/audit-logs", at, "entity_type=employee")
data = d.get("data", {}) if isinstance(d, dict) else {}
check("按 entity_type=employee 过滤 200", resp.status == 200, str(resp.status))
check("entity_type=employee 结果均为 employee", all(r.get("entity_type") == "employee" for r in data.get("list", [])), str([r.get("entity_type") for r in data.get("list", [])]))

print("== 审计查询：分页 ==")
resp, d = get_json("/audit-logs", at, "page=1&size=1")
data = d.get("data", {}) if isinstance(d, dict) else {}
check("分页 size=1 返回 <=1 行", data.get("size") == 1 and len(data.get("list", [])) <= 1, str(data))

# 先解析 admin / demo-sales 员工 id（后续审计过滤与 offboard 均依赖）
resp, d = get_json("/employees", at)
_emp = (d.get("data", {}) or {}).get("list", [])
admin_id = next((e["id"] for e in _emp if e.get("email") == "admin@bss.local"), None)
sales_id = next((e["id"] for e in _emp if e.get("email") == "demo-sales@bss.local"), None)
check("找到 admin 员工 id", admin_id is not None, str(admin_id))
check("找到 demo-sales 员工 id", sales_id is not None, str(sales_id))

print("== 审计查询：offboard 动作（交接前该销售应为 0）==")
resp, d = get_json("/audit-logs", at, "action=offboard")
data = d.get("data", {}) if isinstance(d, dict) else {}
offrows_before = [r for r in data.get("list", []) if r.get("entity_id") == sales_id]
check("交接前该销售无 offboard 审计记录", len(offrows_before) == 0, f"count={len(offrows_before)}")

print("== 离职交接：预览 ==")
resp, d = get_json(f"/employees/{sales_id}/offboard-preview", at)
prev = d.get("data", {}) if isinstance(d, dict) else {}
check("offboard-preview 200", resp.status == 200, str(resp.status))
check("预览显示在职", prev.get("active") is True, str(prev.get("active")))
check("预览显示有数据 has_data", prev.get("has_data") is True, str(prev.get("has_data")))
check("预览客户数 >= 1", prev.get("customers", 0) >= 1, str(prev.get("customers")))
check("预览商单数 >= 1", prev.get("deals", 0) >= 1, str(prev.get("deals")))
check("预览合同数 >= 1", prev.get("contracts", 0) >= 1, str(prev.get("contracts")))

print("== 离职交接：执行转移 ==")
body = json.dumps({"successor_id": admin_id}).encode()
resp, d = req("POST", f"/employees/{sales_id}/offboard", at, body=body)
res = d.get("data", {}) if isinstance(d, dict) else {}
result = res.get("result", {}) if isinstance(res, dict) else {}
check("offboard 200", resp.status == 200, str(resp.status))
check("交接结果含客户/商单/合同计数", all(k in result for k in ("customers", "deals", "contracts")), str(result))
check("交接合同数 >= 1", result.get("contracts", 0) >= 1, str(result.get("contracts")))

print("== 离职交接：转移后校验 ==")
resp, d = get_json(f"/employees/{sales_id}/offboard-preview", at)
prev2 = d.get("data", {}) if isinstance(d, dict) else {}
check("转移后 has_data 为 false", prev2.get("has_data") is False, str(prev2.get("has_data")))
resp, d = get_json("/employees", at)
emp_list2 = (d.get("data", {}) or {}).get("list", [])
sales_now = next((e for e in emp_list2 if e.get("id") == sales_id), None)
check("目标员工已停用 (status=disabled)", sales_now is not None and sales_now.get("status") == "disabled", str(sales_now))

print("== 审计查询：offboard 动作（交接后该销售 >=1）==")
resp, d = get_json("/audit-logs", at, "action=offboard")
data = d.get("data", {}) if isinstance(d, dict) else {}
offrows_after = [r for r in data.get("list", []) if r.get("entity_id") == sales_id and r.get("action") == "offboard"]
check("交接后该销售存在 offboard 审计记录", len(offrows_after) >= 1, f"count={len(offrows_after)}")

print("== 离职交接：守卫 ==")
# 1) 不能交接本人（停自己）
body = json.dumps({"successor_id": admin_id}).encode()
resp, d = req("POST", f"/employees/{admin_id}/offboard", at, body=body)
check("交接本人 → 422", resp.status == 422, str(resp.status))

# 2) 建无业务临时员工：缺失交接人 → 退回公海（200，员工停用）
body = json.dumps({"name": "临时交接测试", "email": "tmp-leaver@bss.local", "dept": "销售一部", "position": "专员", "role": "sales"}).encode()
resp, d = req("POST", "/employees", at, body=body)
tmp_id = (d.get("data", {}) or {}).get("employee", {}).get("id")
check("创建临时员工成功", resp.status == 200 and tmp_id, str(resp.status))
resp, d = req("POST", f"/employees/{tmp_id}/offboard", at, body=json.dumps({}).encode())
check("无业务员工缺失交接人 → 退回公海 200", resp.status == 200, str(resp.status))

# 3) 建有商单的守卫员工：以其自身账号建商单（owner=该员工），缺失交接人 → 422
body = json.dumps({"name": "守卫员工", "email": "guard-leaver@bss.local", "dept": "销售一部", "position": "专员", "role": "sales"}).encode()
resp, d = req("POST", "/employees", at, body=body)
gw_data = (d.get("data", {}) or {})
gw_id = gw_data.get("employee", {}).get("id")
gw_pwd = gw_data.get("initial_password")
check("创建守卫员工成功", resp.status == 200 and gw_id, str(resp.status))
gw_tok = login("guard-leaver@bss.local", gw_pwd)
check("守卫员工登录 200", bool(gw_tok))
resp, d = req("POST", "/customers", gw_tok, body=json.dumps({"name": "守卫客户", "industry": "it", "source": "web", "level": "a"}).encode())
gw_cid = (d.get("data", {}) or {}).get("id")
resp, d = req("POST", "/deals", gw_tok, body=json.dumps({"customer_id": gw_cid, "title": "守卫商单", "amount_cent": 1000}).encode())
check("守卫员工建商单 200（owner=本人）", resp.status == 200, str(resp.status))
# 用 admin 触发离职（缺失交接人）→ 该员工名下有商单，必须 422
resp, d = req("POST", f"/employees/{gw_id}/offboard", at, body=json.dumps({}).encode())
check("有商单缺失交接人 → 422", resp.status == 422, str(resp.status))
# 指定交接人后可正常离职（清理）
resp, d = req("POST", f"/employees/{gw_id}/offboard", at, body=json.dumps({"successor_id": admin_id}).encode())
check("指定交接人后离职 200", resp.status == 200, str(resp.status))

# 4) 交接人非启用（已停用的 demo-sales）→ 422
resp, d = req("POST", f"/employees/{tmp_id}/offboard", at, body=json.dumps({"successor_id": sales_id}).encode())
check("交接人非启用 → 422", resp.status == 422, str(resp.status))

print(f"\n结果: {passed} 通过 / {failed} 失败")
sys.exit(1 if failed else 0)
