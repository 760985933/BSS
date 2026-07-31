import json, urllib.request, urllib.error, os, sys

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip("/")
API = BASE + "/api/v1"
DATA_DIR = "/tmp/bss-contract-test"
FAILED = False


def req(method, path, body=None, token=None, raw=False, files=None):
    url = API + path
    headers = {}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = None
    if files is not None:
        boundary = "----bssboundary"
        parts = []
        for k, fname in files.items():
            with open(fname, "rb") as f:
                content = f.read()
            parts.append(("--%s\r\n" % boundary).encode())
            parts.append(('Content-Disposition: form-data; name="%s"; filename="%s"\r\n' % (k, os.path.basename(fname))).encode())
            parts.append(b"Content-Type: application/octet-stream\r\n\r\n")
            parts.append(content)
            parts.append(b"\r\n")
        parts.append(("--%s--\r\n" % boundary).encode())
        data = b"".join(parts)
        headers["Content-Type"] = "multipart/form-data; boundary=%s" % boundary
    elif body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        resp = urllib.request.urlopen(r)
        b = resp.read()
        if raw:
            return resp.status, b
        if b:
            j = json.loads(b)
            return resp.status, j.get("data", j)
        return resp.status, {}
    except urllib.error.HTTPError as e:
        return e.code, (json.loads(e.read()) if e.fp else {})


def check(name, cond, extra=""):
    global FAILED
    print(("PASS" if cond else "FAIL"), name, extra)
    if not cond:
        FAILED = True


# 登录
st, body = req("POST", "/auth/login", {"email": "admin@bss.local", "password": "admin123"})
check("login", st == 200 and "token" in body, str(st))
token = body["token"]

# 客户
st, c1 = req("POST", "/customers", {"name": "E2E客户A", "industry": "", "source": "", "level": "", "remark": ""}, token)
check("create customer A", st == 200, str(st))
cid1 = c1["id"]
st, c2 = req("POST", "/customers", {"name": "E2E客户B", "industry": "", "source": "", "level": "", "remark": ""}, token)
cid2 = c2["id"]
check("create customer B", st == 200, str(st))

# 建 won 商单
def create_won(cid, title):
    st, d = req("POST", "/deals", {"customer_id": cid, "title": title, "amount_cent": 100000,
                                   "probability": 10, "expected_sign_date": "", "remark": ""}, token)
    if st != 200:
        return None
    did = d["id"]
    for to in ["qualifying", "proposal", "negotiating"]:
        req("POST", "/deals/%s/status" % did, {"to": to}, token)
    # 经审批流到达 won（普通 ChangeStatus 不允许 negotiating→won，须经折扣审批）
    st, ap = req("POST", "/approvals", {"entity_type": "deal", "entity_id": did,
                                        "kind": "deal_discount", "amount_cent": 0}, token)
    if st == 200:
        apid = ap.get("id")
        req("POST", "/approvals/%s/approve" % apid, token=token)
    return did

d1 = create_won(cid1, "E2E商单A")
d2 = create_won(cid2, "E2E商单B")
check("create won deal A", d1 is not None)
check("create won deal B", d2 is not None)

# 同客户关联 won 商单 -> 成功
st, ct1 = req("POST", "/contracts", {"customer_id": cid1, "title": "E2E合同", "amount_cent": 50000,
                                     "deal_ids": [int(d1)]}, token)
check("create contract (same customer)", st == 200, "status=%s body=%s" % (st, ct1))
ctid = ct1["id"]
check("contract code HT-", ct1.get("code", "").startswith("HT-"), ct1.get("code", ""))

# 跨客户关联 won 商单 -> 422
st, ct2 = req("POST", "/contracts", {"customer_id": cid1, "title": "跨客户", "amount_cent": 1,
                                     "deal_ids": [int(d2)]}, token)
check("cross-customer deal rejected 422", st == 422, "status=%s" % st)

# 状态流转 draft->pending->(审批)->signed
req("POST", "/contracts/%s/status" % ctid, {"to": "pending"}, token)
st, ap = req("POST", "/approvals", {"entity_type": "contract", "entity_id": ctid, "kind": "contract_sign"}, token)
if st == 200:
    st, _ = req("POST", "/approvals/%s/approve" % ap.get("id"), token=token)
st, sc = req("GET", "/contracts/%s" % ctid, token=token)
check("contract signed", st == 200 and sc.get("status") == "signed", "status=%s" % (sc.get("status") if isinstance(sc, dict) else sc))
check("signed status", isinstance(sc, dict) and sc.get("status") == "signed", sc.get("status", "") if isinstance(sc, dict) else "")

# 终态锁定：signed 后改金额 -> 422
st, _ = req("PUT", "/contracts/%s" % ctid, {"customer_id": cid1, "title": "X", "amount_cent": 99999}, token)
check("amount locked after signed 422", st == 422, "status=%s" % st)

# 终态锁定：signed 后改关联商单 -> 422
st, _ = req("PUT", "/contracts/%s/deals" % ctid, {"deal_ids": [int(d1)]}, token)
check("deals locked after signed 422", st == 422, "status=%s" % st)

# terminated 必填原因
req("POST", "/contracts/%s/status" % ctid, {"to": "performing"}, token)
st, _ = req("POST", "/contracts/%s/status" % ctid, {"to": "terminated"}, token)
check("terminated requires reason 422", st == 422, "status=%s" % st)
st, _ = req("POST", "/contracts/%s/status" % ctid, {"to": "terminated", "terminate_reason": "客户违约"}, token)
check("terminated with reason ok", st == 200, "status=%s" % st)

# 附件上传/下载/鉴权
fpath = "/tmp/bss_test_upload.pdf"
with open(fpath, "wb") as f:
    f.write(b"%PDF-1.4 test contract attachment content")
st, att = req("POST", "/contracts/%s/attachments" % ctid, files={"file": fpath}, token=token)
check("attachment upload 200", st == 200, "status=%s body=%s" % (st, att))
aid = att.get("id")
st, content = req("GET", "/attachments/%s/download" % aid, token=token, raw=True)
check("attachment download 200 + content", st == 200 and b"test contract attachment" in content, "status=%s" % st)
st, _ = req("GET", "/attachments/%s/download" % aid, raw=True)
check("attachment download requires auth 401", st == 401, "status=%s" % st)
st, _ = req("DELETE", "/attachments/%s" % aid, token=token)
check("attachment delete 200", st == 200, "status=%s" % st)

print("RESULT:", "ALL PASS" if not FAILED else "HAS FAILURES")
sys.exit(1 if FAILED else 0)
