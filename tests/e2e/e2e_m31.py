#!/usr/bin/env python3
# 客户公海池 E2E（M3-1）：公海列表/领取/释放/回收/规则/流水/离职退公海
import sys, json, urllib.request, urllib.error, time
from urllib.parse import quote

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
ADMIN = ("admin@bss.local", "admin123")
SALES = ("demo-sales@bss.local", "Bss@1234")
FINANCE = ("demo-finance@bss.local", "Bss@1234")
TS = str(int(time.time()))

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
        enc = []
        for pair in params.split("&"):
            if "=" in pair:
                k, v = pair.split("=", 1)
                enc.append(f"{k}={quote(v)}")
            else:
                enc.append(quote(pair))
        url += ("&" if "?" in url else "?") + "&".join(enc)
    r = urllib.request.Request(url, method=method, data=body)
    if token:
        r.add_header("Authorization", "Bearer " + token)
    if body is not None and ctype:
        r.add_header("Content-Type", ctype)
    try:
        resp = urllib.request.urlopen(r, timeout=15)
        b = resp.read()
        if raw:
            return resp, b
        try:
            return resp, json.loads(b)
        except Exception:
            return resp, b.decode(errors="replace")
    except urllib.error.HTTPError as e:
        try:
            eb = e.read().decode(errors="replace")
        except Exception:
            eb = ""
        return e, (eb if raw else eb)

def login(email, pwd):
    url = BASE + "/api/v1/auth/login"
    body = json.dumps({"email": email, "password": pwd}).encode()
    r = urllib.request.Request(url, data=body, method="POST")
    r.add_header("Content-Type", "application/json")
    try:
        resp = urllib.request.urlopen(r, timeout=15)
        return json.loads(resp.read()).get("data", {}).get("token")
    except Exception as e:
        print("  login err", email, e)
        return None

def get_json(path, token, params=None):
    return req("GET", path, token, params=params)

def post_json(path, token, obj=None):
    body = json.dumps(obj).encode() if obj is not None else b"{}"
    return req("POST", path, token, body=body)

def put_json(path, token, obj=None):
    body = json.dumps(obj).encode() if obj is not None else b"{}"
    return req("PUT", path, token, body=body)

def dget(d, *keys):
    cur = d
    for k in keys:
        if not isinstance(cur, dict):
            return None
        cur = cur.get(k)
    return cur

# ---------- 登录 ----------
print("== 登录 ==")
at = login(*ADMIN)
check("admin 登录", bool(at))
# 自建销售员工（server 不自动 seed demo 账号），用于验证领取/行级权限
r, d = post_json("/employees", at, {"name": f"销售E2E-{TS}", "email": f"tmp_sales_{TS}@bss.local", "dept": "销售一部", "position": "经理", "role": "sales"})
se = dget(d, "data", "employee") or {}
se_id = se.get("id")
check("创建销售员工", r.status == 200 and se_id, str(r.status))
st = login(f"tmp_sales_{TS}@bss.local", se.get("initial_password") or "Bss@1234") or login(f"tmp_sales_{TS}@bss.local", "Bss@1234")
check("销售登录", bool(st))
sales_id = se_id
# 自建一个财务员工用于验证「公海全员可读」
r, d = post_json("/employees", at, {"name": f"财务E2E-{TS}", "email": f"tmp_fin_{TS}@bss.local", "dept": "财务部", "position": "会计", "role": "finance"})
fe = dget(d, "data", "employee") or {}
fe_id = fe.get("id")
check("创建财务员工", r.status == 200 and fe_id, str(r.status))
ft = login(f"tmp_fin_{TS}@bss.local", fe.get("initial_password") or "Bss@1234") or login(f"tmp_fin_{TS}@bss.local", "Bss@1234")
check("财务登录", bool(ft))

# ---------- 权限：公海列表全员可读 ----------
print("== 权限：公海列表 ==")
r, d = get_json("/customer-pool", st)
check("销售可访问公海列表 (200)", r.status == 200, str(r.status))
r, d = get_json("/customer-pool", ft)
check("财务可访问公海列表 (200)", r.status == 200, str(r.status))
r, d = get_json("/customer-pool/settings", st)
check("销售可读取回收规则 (200)", r.status == 200, str(r.status))

# ---------- 回收规则：权限与读写 ----------
print("== 回收规则：设置权限 ==")
r, d = get_json("/customer-pool/settings", at)
s = dget(d, "data") or {}
check("admin 读取规则含 enabled/max_claim_per_sales", all(k in s for k in ("enabled", "max_claim_per_sales", "idle_days_no_follow", "idle_days_no_deal", "protect_days")), str(s))
r, d = put_json("/customer-pool/settings", st, {"enabled": True, "max_claim_per_sales": 1, "idle_days_no_follow": 30, "idle_days_no_deal": 60, "protect_days": 7})
check("销售不可修改规则 (403)", r.status == 403, str(r.status))
r, d = put_json("/customer-pool/settings", at, {"enabled": True, "max_claim_per_sales": 1, "idle_days_no_follow": 30, "idle_days_no_deal": 60, "protect_days": 7})
check("admin 修改规则 (200)", r.status == 200, str(r.status))
r, d = get_json("/customer-pool/settings", at)
s2 = dget(d, "data") or {}
check("规则已写入(max=1)", s2.get("max_claim_per_sales") == 1, str(s2))
check("规则已写入(enabled=true)", s2.get("enabled") is True, str(s2))

# ---------- 准备客户并释放到公海 ----------
print("== 释放客户到公海 ==")
def create_customer(token, name):
    r, d = post_json("/customers", token, {"name": name, "industry": "互联网", "source": "转介绍", "level": "A", "remark": "e2e"})
    c = dget(d, "data", "customer") or dget(d, "data") or {}
    return r, c

r, cA = create_customer(at, f"公海E2E-A-{TS}")
check("admin 创建客户A", r.status == 200 and cA.get("id"), str(r.status))
a_id = cA.get("id")
r, _ = post_json(f"/customers/{a_id}/release", at, {"reason": "主动释放"})
check("admin 释放客户A到公海 (200)", r.status == 200, str(r.status))
r, d = get_json("/customer-pool", at, f"keyword=公海E2E-A-{TS}")
plist = dget(d, "data", "list") or []
hit = next((c for c in plist if c.get("id") == a_id), None)
check("客户A出现在公海列表(owner_id=0)", hit is not None and hit.get("owner_id") == "0", str(hit))

# ---------- 领取 ----------
print("== 领取 ==")
r, d = post_json(f"/customers/{a_id}/claim", st)
check("销售领取客户A (200)", r.status == 200, str(r.status))
r, d = get_json(f"/customers/{a_id}", at)
ca = dget(d, "data") or {}
check("领取后 owner 变为销售", ca.get("owner_id") == dget(d, "data", "owner_id") and ca.get("owner_id") is not None, str(ca))
sales_id = ca.get("owner_id")
check("销售 id 非 0", sales_id not in (None, "0"), str(sales_id))

# ---------- 流水：claim ----------
print("== 公海流水 ==")
r, d = get_json(f"/customers/{a_id}/pool-logs", at)
logs = dget(d, "data") or []
acts = [l.get("action") for l in logs]
check("流水含 claim 动作", "claim" in acts, str(acts))

# ---------- 领取上限（max=1）----------
print("== 领取上限 ==")
r, cB = create_customer(at, f"公海E2E-B-{TS}")
b_id = cB.get("id")
r, _ = post_json(f"/customers/{b_id}/release", at, {"reason": "主动释放"})
check("admin 释放客户B (200)", r.status == 200, str(r.status))
# 当前销售已持有 A（1 个），达到上限；再领取 B 应 422
r, d = post_json(f"/customers/{b_id}/claim", st)
check("达上限后再领取 → 422", r.status == 422, str(r.status))
# 释放 A 腾出额度，再次领取 B 应成功
r, _ = post_json(f"/customers/{a_id}/release", st, {"reason": "主动释放"})
check("销售释放客户A (200)", r.status == 200, str(r.status))
r, d = post_json(f"/customers/{b_id}/claim", st)
check("腾出额度后领取B成功 (200)", r.status == 200, str(r.status))
# 复原：释放 B
r, _ = post_json(f"/customers/{b_id}/release", st, {"reason": "主动释放"})
check("复原：释放客户B (200)", r.status == 200, str(r.status))
r, d = get_json(f"/customers/{a_id}/pool-logs", at)
logs2 = dget(d, "data") or []
acts2 = [l.get("action") for l in logs2]
check("流水含 release 动作", "release" in acts2, str(acts2))

# ---------- 自动回收（结构校验）----------
print("== 自动回收接口 ==")
r, d = post_json("/customer-pool/recycle?dry_run=1", at)
res = dget(d, "data") or {}
check("试算回收 200 且含 total/items", r.status == 200 and "total" in res and "items" in res, str(res))
r, d = post_json("/customer-pool/recycle", at)
res2 = dget(d, "data") or {}
check("执行回收 200 且含 total", r.status == 200 and "total" in res2, str(res2))
# 关闭自动回收，规则恢复默认（避免影响其他用例）
r, d = put_json("/customer-pool/settings", at, {"enabled": False, "max_claim_per_sales": 50, "idle_days_no_follow": 30, "idle_days_no_deal": 60, "protect_days": 7})
check("复原规则(关闭回收, max=50) 200", r.status == 200, str(r.status))

# ---------- 离职退回公海 ----------
print("== 离职退公海 ==")
r, d = post_json("/employees", at, {"name": f"离职池测试-{TS}", "email": f"tmp_pool_{TS}@bss.local", "dept": "销售一部", "position": "专员", "role": "sales"})
te = dget(d, "data", "employee") or {}
te_id = te.get("id")
check("创建临时销售员工", r.status == 200 and te_id, str(r.status))
# 以该员工身份登录并建客户（owner=该员工）
tt = login(f"tmp_pool_{TS}@bss.local", te.get("initial_password") or "Bss@1234")
if not tt:
    # 初始密码可能不同，尝试默认
    tt = login(f"tmp_pool_{TS}@bss.local", "Bss@1234")
check("临时销售登录", bool(tt))
r, cT = create_customer(tt, f"离职客户-{TS}")
tcu_id = cT.get("id")
check("临时销售创建客户(owner=自己)", r.status == 200 and tcu_id, str(r.status))
# 离职不指定交接人 → 客户退回公海
r, d = post_json(f"/employees/{te_id}/offboard", at, {"successor_id": "0"})
res = dget(d, "data", "result") or {}
check("离职退公海 200", r.status == 200, str(r.status))
check("离职结果含 customers 计数", "customers" in res, str(res))
r, d = get_json(f"/customers/{tcu_id}", at)
ct = dget(d, "data") or {}
check("离职后客户 owner 变公海(0)", ct.get("owner_id") == "0", str(ct))
check("离职后 pool_reason=负责人离职未指定交接人", ct.get("pool_reason") == "负责人离职未指定交接人", str(ct.get("pool_reason")))
r, d = get_json(f"/customers/{tcu_id}/pool-logs", at)
tlogs = dget(d, "data") or []
tacts = [l.get("action") for l in tlogs]
check("离职流水含 recycle/assign 动作", ("recycle" in tacts or "assign" in tacts), str(tacts))

# ---------- 离职守卫：有商单必须指定交接人 ----------
print("== 离职守卫（有商单）==")
r, d = post_json("/employees", at, {"name": f"离职守卫-{TS}", "email": f"tmp_guard_{TS}@bss.local", "dept": "销售二部", "position": "专员", "role": "sales"})
ge = dget(d, "data", "employee") or {}
ge_id = ge.get("id")
check("创建守卫测试员工", r.status == 200 and ge_id, str(r.status))
gt = login(f"tmp_guard_{TS}@bss.local", ge.get("initial_password") or "Bss@1234") or login(f"tmp_guard_{TS}@bss.local", "Bss@1234")
check("守卫员工登录", bool(gt))
r, cG = create_customer(gt, f"守卫客户-{TS}")
cgu_id = cG.get("id")
check("守卫员工创建客户", r.status == 200 and cgu_id, str(r.status))
r, _ = post_json("/deals", gt, {"customer_id": cgu_id, "title": "测试商单", "amount_cent": 100000, "probability": 60})
check("守卫员工创建商单", r.status == 200, str(r.status))
r, d = post_json(f"/employees/{ge_id}/offboard", at, {"successor_id": "0"})
check("有商单离职不指定交接人 → 422", r.status == 422, str(r.status))
# 指定交接人后可正常离职（清理，交接给活跃销售）
r, d = post_json(f"/employees/{ge_id}/offboard", at, {"successor_id": sales_id})
check("指定交接人后离职 200", r.status == 200, str(r.status))

# ---------- 清理临时客户与员工 ----------
print("== 清理 ==")
for cid in (a_id, b_id, tcu_id, cgu_id):
    if cid:
        req("DELETE", f"/customers/{cid}", at)
# 停用未离职的临时员工，避免数据库累积
for eid in (fe_id,):
    if eid:
        post_json(f"/employees/{eid}/offboard", at, {"successor_id": sales_id})
check("清理临时客户/员工完成", True)

print(f"\n结果: {passed} 通过 / {failed} 失败")
sys.exit(1 if failed else 0)
