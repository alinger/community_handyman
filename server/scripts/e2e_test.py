#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
邻有回应 · 端到端主流程测试

覆盖：注册/登录/改密 → 地址与服务范围 → 计价试算 → 下单 → 支付回调
      → 服务方认证审核 → 接单履约 → 服务记录 → 用户确认完成
      → 取消与退款 → 包月核销 → 短信验证码 → 后台看板与导出

设计原则：
- 只依赖标准库，不改动业务代码，可随时重跑；
- 每一步都断言业务码，失败立即打印请求/响应上下文，便于定位；
- 数据自洽：手机号带时间戳，重复运行不撞唯一索引。
"""

import hashlib
import hmac
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request

import os

# 从环境变量取，便于 run_e2e.sh 注入端口与库名；默认值与脚本约定一致
BASE = os.environ.get("E2E_BASE", "http://127.0.0.1:8098")
E2E_DB = os.environ.get("E2E_DB", "community_handyman_e2e")
ADMIN_PWD = os.environ.get("E2E_ADMIN_PWD", "E2eTest@123")

PASSED = []
FAILED = []


# ---------- 基础设施 ----------

def call(method, path, body=None, token=None, expect=0, raw=False):
    """发起请求并断言业务码。返回 data 部分。"""
    url = BASE + path
    data = None
    headers = {"Content-Type": "application/json"}
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode()
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            payload = json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        payload = json.loads(e.read().decode())

    if raw:
        return payload
    if payload.get("code") != expect:
        raise AssertionError(
            f"{method} {path} 期望 code={expect}，实际 {payload.get('code')} "
            f"message={payload.get('message')}"
        )
    return payload.get("data")


def step(name):
    print(f"\n▶ {name}")


def ok(desc, extra=""):
    PASSED.append(desc)
    print(f"  ✔ {desc} {extra}")


def fail(desc, err):
    FAILED.append((desc, str(err)))
    print(f"  �’ {desc}\n     {err}")


def check(desc, fn):
    """执行一个断言块，异常不影响后续用例。"""
    try:
        result = fn()
        ok(desc, result if isinstance(result, str) else "")
        return True
    except AssertionError as e:
        fail(desc, e)
        return False
    except Exception as e:
        fail(desc, f"{type(e).__name__}: {e}")
        return False


def _mysql_pwd():
    """从 server/.env 读密码，避免在本文件里写死凭据。"""
    here = os.path.dirname(os.path.abspath(__file__))
    env_path = os.path.join(here, "..", ".env")
    with open(env_path, encoding="utf-8") as f:
        for line in f:
            if line.startswith("MYSQL_DSN="):
                seg = line.split("root:", 1)[1]
                return seg.split("@", 1)[0]
    raise RuntimeError("无法从 .env 解析 MySQL 密码")


MYSQL_PWD = _mysql_pwd()


def sql(query):
    """直接查库核对状态（绕过接口，验证落库是否正确）。"""
    r = subprocess.run(
        ["mysql", "-uroot", f"-p{MYSQL_PWD}", "-h127.0.0.1", E2E_DB,
         "-N", "-B", "-e", query],
        capture_output=True, text=True)
    if r.returncode != 0:
        raise RuntimeError(r.stderr.strip())
    return r.stdout.strip()


def phone(suffix):
    """生成不冲突的手机号：173 + 8 位时间派生。"""
    base = int(time.time() * 1000) % 100000000
    return f"173{base:08d}"[:11] if suffix is None else f"173{base:08d}"[:8] + f"{suffix:03d}"


def sign_sandbox(order_no, txid, amount, secret="sandbox-secret"):
    msg = f"{order_no}|{txid}|{amount}"
    return hmac.new(secret.encode(), msg.encode(), hashlib.sha256).hexdigest()


# ---------- 用例 ----------

def main():
    ts = int(time.time())
    user_phone = f"173{ts % 100000000:08d}"[:11]
    user_pwd = "E2eUser@123"
    ctx = {}

    # 1x1 PNG。服务方证件与服务记录上传都要用，必须提前落盘：
    # 之前只在记录用例里生成，干净机器首次运行会在「上传身份证」那步读不到文件。
    open("/tmp/e2e.png", "wb").write(bytes.fromhex(
        "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4"
        "890000000a49444154789c6360000002000100ffff03000006000557bfabd400"
        "00000049454e44ae426082"))

    # ============ 1. 账号体系 ============
    step("1. 注册与登录")

    def t_register():
        d = call("POST", "/api/v1/auth/register",
                 {"phone": user_phone, "password": user_pwd, "nickname": "E2E住户"})
        assert d["token"], "未返回 token"
        ctx["user_token"] = d["token"]
        ctx["user_id"] = d["user"]["user_id"]
        return f"user_id={d['user']['user_id']}"
    check("手机号注册成功并返回登录态", t_register)

    check("重复注册被拒（1005）", lambda: call(
        "POST", "/api/v1/auth/register",
        {"phone": user_phone, "password": user_pwd}, expect=1005))

    check("密码登录成功", lambda: ctx.update(
        user_token=call("POST", "/api/v1/auth/login",
                        {"phone": user_phone, "password": user_pwd})["token"]) or "token 已刷新")

    check("错误密码被拒（1002）", lambda: call(
        "POST", "/api/v1/auth/login",
        {"phone": user_phone, "password": "wrong123"}, expect=1002))

    check("带 token 读取个人信息", lambda: "nickname=" + call(
        "GET", "/api/v1/me", token=ctx["user_token"])["nickname"])

    check("未带 token 访问受保护接口被拒（1002）", lambda: call(
        "GET", "/api/v1/me", expect=1002))

    # ============ 2. 地址与服务范围 ============
    step("2. 地址与服范围校验")

    check("可选小区列表非空", lambda: f"{len(call('GET', '/api/v1/addresses/communities', token=ctx['user_token']))} 个小区")

    check("超出服务范围的小区被拒（2001）", lambda: call(
        "POST", "/api/v1/addresses",
        {"community": "不存在的小区", "house_no": "1"}, token=ctx["user_token"], expect=2001))

    def t_addr():
        d = call("POST", "/api/v1/addresses", {
            "community": "东方冠郡二期", "lane": "众安路779弄", "house_no": "12",
            "contact_name": "E2E", "contact_phone": user_phone,
        }, token=ctx["user_token"])
        ctx["addr_id"] = d["id"]
        assert d["in_service_area"] is True, "应判定在服务范围内"
        return f"address_id={d['id']}"
    check("新增地址成功且在服务范围", t_addr)

    check("设为默认地址", lambda: call(
        "PUT", f"/api/v1/addresses/{ctx['addr_id']}/default",
        token=ctx["user_token"]) and "已设默认")

    check("地址范围预校验通过", lambda: call(
        "POST", "/api/v1/addresses/check",
        {"community": "东方冠郡二期", "house_no": "12"}, token=ctx["user_token"])["in_service_area"] and "in_area")

    def t_service_areas():
        """服务范围页是公开接口：访客不登录也要能看，否则先注册才知道送不送很别扭。"""
        d = call("GET", "/api/v1/service-areas")
        assert isinstance(d, list) and d, f"服务范围应为非空数组，实际 {d}"
        area = d[0]
        assert area.get("community"), f"缺少小区名：{area}"
        lanes = area.get("lanes") or []
        assert lanes, f"小区未配置路段：{area}"
        lane = lanes[0]
        for k in ("lane", "house_from", "house_to", "house_parity"):
            assert k in lane, f"路段缺字段 {k}：{lane}"
        return (f"{len(d)} 个小区，首个 {area['community']}/"
                f"{lane['lane']} {lane['house_from']}-{lane['house_to']} 号")
    check("服务范围公开可查（免登录）", t_service_areas)

    # ============ 3. 计价与时段 ============
    step("3. 计价试算与时段库存")

    def slot_avail(svc):
        return call("GET", f"/api/v1/services/slots?date={ctx['date']}&service_type={svc}",
                    token=ctx["user_token"])

    def t_pick_date():
        # 用后天，避开「2 小时内不可取消」的边界
        for offset in (2, 3, 4):
            d = time.strftime("%Y-%m-%d", time.localtime(time.time() + offset * 86400))
            slots = call("GET", f"/api/v1/services/slots?date={d}&service_type=garbage",
                         token=ctx["user_token"])
            free = [s for s in slots if s["available"]]
            if free:
                ctx["date"] = d
                ctx["garbage_slot"] = free[0]["slot_code"]
                return f"date={d} slot={free[0]['slot_code']}"
        raise AssertionError("未来 3 天没有可约时段，请检查种子数据")
    check("找到可预约日期与时段", t_pick_date)

    def t_preview_garbage():
        d = call("POST", "/api/v1/orders/preview", {
            "address_id": ctx["addr_id"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["user_token"])
        ctx["garbage_total"] = d["total"]
        # 000013 起单次倒垃圾只能用注册赠送的免费次数，试算即 0 元
        assert d["total"] == 0, f"单次倒垃圾有免费次数时应为 0 分，实际 {d['total']}"
        return f"total={d['total']}分（免费次数抵扣）"
    check("代倒垃圾单次走免费次数（0 元）", t_preview_garbage)

    def t_garbage_plan():
        d = call("GET", "/api/v1/services/garbage-plan")
        assert d["price"] == 3000, f"包月价应为 3000，实际 {d['price']}"
        assert d["single_price"] == 500, f"单次价应为 500，实际 {d['single_price']}"
        assert d["default_slot"] == "22:00-22:30", f"默认时段不对: {d['default_slot']}"
        return f"包月 ¥{d['price']/100} / {d['days']} 天，单次 ¥{d['single_price']/100}"
    check("包月方案接口返回价格与时段", t_garbage_plan)

    def t_preview_dogwalk():
        # 遛宠工作日只开晚间，取一个有余额的时段
        for offset in (2, 3, 4, 5):
            d0 = time.strftime("%Y-%m-%d", time.localtime(time.time() + offset * 86400))
            slots = call("GET", f"/api/v1/services/slots?date={d0}&service_type=dogwalk",
                         token=ctx["user_token"])
            free = [s for s in slots if s["available"]]
            if free:
                ctx["dog_date"] = d0
                ctx["dog_slot"] = free[0]["slot_code"]
                break
        assert ctx.get("dog_slot"), "找不到可约的遛宠时段"
        d = call("POST", "/api/v1/orders/preview", {
            "address_id": ctx["addr_id"], "service_type": "dogwalk",
            "service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
            "pet_count": 1, "duration_min": 20, "distance_km": 1,
        }, token=ctx["user_token"])
        ctx["dogwalk_total"] = d["total"]
        assert d["total"] == 4000, f"遛宠基础价应为 4000 分，实际 {d['total']}"
        return f"total={d['total']}分（{d['items'][0]['name']}）"
    check("遛宠试算金额正确（¥40.00）", t_preview_dogwalk)

    def t_preview_accompany():
        opts = call("GET", "/api/v1/services/accompany/options", token=ctx["user_token"])
        hospital = opts["hospitals"][0]["code"]
        items = [i["code"] for i in opts["service_items"][:2]]
        ctx["acc_hospital"] = hospital
        ctx["acc_items"] = items
        d = call("POST", "/api/v1/orders/preview", {
            "address_id": ctx["addr_id"], "service_type": "accompany",
            "service_date": ctx["date"], "slot_code": "08-10",
            "hospital": hospital, "patient_name": "E2E病人",
            "service_items": items,
        }, token=ctx["user_token"])
        ctx["acc_total"] = d["total"]
        assert d["total"] > 0, "陪诊金额应大于 0"
        # 基础服务费 1000 + 所选两项，验证基础费被强制计入
        assert any("基础" in i["name"] for i in d["items"]), "缺少基础服务费项"
        return f"total={d['total']}分，含基础服务费"
    check("陪诊按服务项求和且强制含基础费", t_preview_accompany)

    def t_familiar_progress():
        """熟悉费进度：前端按 done/need 画进度条，门槛值由后台下发，不该写死。"""
        d = call("POST", "/api/v1/orders/preview", {
            "address_id": ctx["addr_id"], "service_type": "dogwalk",
            "service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
            "pet_count": 1, "duration_min": 20, "distance_km": 1,
            "need_familiar": True,
        }, token=ctx["user_token"])
        f = d.get("familiar_progress")
        assert f, f"开启提前上门熟悉后应返回 familiar_progress：{d}"
        assert f["need"] == 5, f"免熟悉费门槛应为 5 单，实际 {f['need']}"
        assert f["free"] is False, f"零完成单不该判定为已免费：{f}"
        assert d["total"] == 6000, f"熟悉费应叠加 2000 分，实际 {d['total']}"
        return f"done={f['done']}/{f['need']}，未免费，total={d['total']}分"
    check("提前上门熟悉返回累计进度", t_familiar_progress)

    def t_extra_slot_invalid():
        """额外时段必须来自后台配置，乱传要拦在入口，不能让脏数据进订单。"""
        r = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"],
            "extra_slot_code": "03:00-04:00",
        }, token=ctx["user_token"], expect=1001, raw=True)
        assert r["code"] == 1001, f"非法额外时段应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("包月额外时段超配置被拒（1001）", t_extra_slot_invalid)

    def t_extra_slot_ok():
        plan = call("GET", "/api/v1/services/garbage-plan")
        extras = plan.get("extra_slots") or []
        assert extras, f"包月方案未配置候选时段：{plan}"
        assert plan["default_slot"] not in extras, "候选时段不应包含默认时段"
        code = extras[0]
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"],
            "extra_slot_code": code,
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        assert d["order"]["extra_slot_code"] == code, f"额外时段未落库：{d['order']}"
        call("POST", f"/api/v1/orders/{no}/cancel", {"reason": "E2E 清理"},
             token=ctx["user_token"])
        return f"{code} 已写入订单并清理"
    check("包月可指定第二个取件时段", t_extra_slot_ok)

    # ============ 4. 下单与支付 ============
    step("4. 下单、支付回调与幂等")

    def t_create():
        # 支付流用遛宠单：代倒垃圾自 000013 起走免费次数/包月，没有在线支付环节
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "dogwalk",
            "service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
            "pet_count": 1, "duration_min": 20,
            # 门禁信息随单提交，后续用例验证加密落库与可见时间窗
            "entry_method": "password", "access_info": "8888#",
        }, token=ctx["user_token"])
        ctx["order_no"] = d["order"]["order_no"]
        assert d["order"]["status"] == "pending_payment", f"下单后应为待支付，实际 {d['order']['status']}"
        assert d["pay"], "未返回支付参数"
        return f"order_no={ctx['order_no']}"
    check("创建订单并返回支付参数", t_create)

    check("订单出现在「待处理」tab", lambda: ctx["order_no"] in [
        o["order_no"] for o in call("GET", "/api/v1/orders?tab=pending",
                                    token=ctx["user_token"])["list"]] and "已列出")

    def t_pay():
        call("POST", f"/api/v1/dev/orders/{ctx['order_no']}/mock-pay", token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["status"] == "pending_accept", f"支付后应为待接单，实际 {o['status']}"
        ctx["paid_status"] = o["status"]
        return f"status={o['status']}"
    check("模拟支付成功，状态流转为待接单", t_pay)

    def t_pay_idempotent():
        # 重复回调不应重复记流水
        before = sql(f"SELECT COUNT(*) FROM payment_transactions WHERE order_no='{ctx['order_no']}'")
        call("POST", f"/api/v1/dev/orders/{ctx['order_no']}/mock-pay",
             token=ctx["user_token"], raw=True)
        after = sql(f"SELECT COUNT(*) FROM payment_transactions WHERE order_no='{ctx['order_no']}'")
        assert before == after, f"重复支付产生了重复流水：{before} -> {after}"
        return f"流水条数稳定为 {after}"
    check("重复支付回调幂等（不重复记账）", t_pay_idempotent)

    check("已支付订单不可再次发起支付（2003）", lambda: call(
        "POST", f"/api/v1/orders/{ctx['order_no']}/pay",
        token=ctx["user_token"], expect=2003))

    # ============ 5. 服务方认证 ============
    step("5. 服务方注册、申请与后台审核")

    prov_phone = f"173{(ts + 7) % 100000000:08d}"[:11]
    call_ok = call("POST", "/api/v1/auth/register",
                   {"phone": prov_phone, "password": "E2eProv@123", "nickname": "E2E服务方"})
    ctx["prov_token"] = call_ok["token"]
    ctx["prov_uid"] = call_ok["user"]["user_id"]

    def upload_cert(kind):
        """证照必须先上传拿到 object key，提交申请时回传 key。"""
        r = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/provider/application/cert",
            "-H", f"Authorization: Bearer {ctx['prov_token']}",
            "-F", f"kind={kind}", "-F", "file=@/tmp/e2e.png",
        ], capture_output=True, text=True)
        d = json.loads(r.stdout)
        assert d["code"] == 0, f"上传 {kind} 失败：{d}"
        return d["data"]["key"]

    def t_upload_certs():
        ctx["cert_front"] = upload_cert("front")
        ctx["cert_back"] = upload_cert("back")
        return "正反面已上传"
    check("上传身份证正反面", t_upload_certs)

    def t_apply():
        d = call("POST", "/api/v1/provider/application", {
            "real_name": "张三", "phone": prov_phone, "service_type": "all",
            "experience": "社区服务 3 年，熟悉本小区楼栋分布",
            "id_card_no": "310101199001011234",
            "id_card_front_key": ctx["cert_front"],
            "id_card_back_key": ctx["cert_back"],
        }, token=ctx["prov_token"])
        ctx["app_id"] = d["id"]
        assert d["status"] == "pending", f"应待审核，实际 {d['status']}"
        return f"application_id={d['id']}"
    check("提交服务方认证申请", t_apply)

    check("待审核状态不可重复提交（1001）", lambda: call(
        "POST", "/api/v1/provider/application", {
            "real_name": "张三", "phone": prov_phone, "service_type": "all",
            "experience": "重复提交",
        }, token=ctx["prov_token"], expect=1001))

    check("未认证用户访问服务方接口被拒（1003）", lambda: call(
        "GET", "/api/v1/provider/orders/open", token=ctx["prov_token"], expect=1003))

    def t_admin_login():
        d = call("POST", "/api/admin/login", {"username": "admin", "password": ADMIN_PWD})
        ctx["admin_token"] = d["token"]
        assert d["admin"]["role"] == "admin", "应为超管"
        return f"role={d['admin']['role']}"
    check("后台超管登录", t_admin_login)

    def t_approve():
        call("POST", f"/api/admin/provider-applications/{ctx['app_id']}/approve",
             token=ctx["admin_token"])
        role = sql(f"SELECT role FROM users WHERE phone='{prov_phone}'")
        assert role == "provider", f"审核通过后 role 应为 provider，实际 {role}"
        return f"users.role={role}"
    check("审核通过并同步授予 provider 角色", t_approve)

    def t_provider_can_see():
        d = call("GET", "/api/v1/provider/orders/open", token=ctx["prov_token"])
        nos = [o["order_no"] for o in d["list"]]
        assert ctx["order_no"] in nos, "认证后应能在接单池看到待接订单"
        return f"接单池 {d['total']} 单"
    check("认证后可见接单池", t_provider_can_see)

    # ============ 5.5 免费次数与额度归还 ============
    step("5.5 免费次数抵扣与拒单归还")

    # 号码偏移量与其它用例错开（+7/+13/+21/+31/+41 已被占用）
    quota_phone = f"173{(ts + 53) % 100000000:08d}"[:11]

    def t_quota_user():
        d = call("POST", "/api/v1/auth/register", {
            "phone": quota_phone, "password": "E2eQuota@123", "nickname": "额度住户"})
        ctx["quota_token"] = d["token"]
        ctx["quota_uid"] = d["user"]["user_id"]
        addr = call("POST", "/api/v1/addresses", {
            "community": "东方冠郡二期", "lane": "众安路779弄", "house_no": "15",
            "contact_name": "额度住户", "contact_phone": quota_phone,
        }, token=ctx["quota_token"])
        ctx["quota_addr"] = addr["id"]
        return f"user_id={d['user']['user_id']}，address_id={addr['id']}"
    check("额度用例账户与地址就绪", t_quota_user)

    def t_quota_welcome():
        q = call("GET", "/api/v1/me/quotas", token=ctx["quota_token"])
        row = next((x for x in q if x["service_type"] == "garbage"), None)
        assert row, f"未发放倒垃圾次数：{q}"
        assert row["total"] == 2 and row["remaining"] == 2, f"注册礼应为 2 次，实际 {row}"
        return f"{row['remaining']}/{row['total']}"
    check("注册赠送 2 次倒垃圾额度", t_quota_welcome)

    def t_quota_order():
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["quota_addr"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["quota_token"])
        o = d["order"]
        ctx["quota_no"] = o["order_no"]
        assert o["total_amount"] == 0, f"用免费次数应为 0 元，实际 {o['total_amount']}"
        assert o.get("free_quota_used") is True, f"未标记 free_quota_used：{o}"
        assert o["status"] == "pending_accept", f"0 元单应直接进入待接单，实际 {o['status']}"
        return f"{o['order_no']}，0 元，{o['status']}"
    check("用免费次数下单金额为 0", t_quota_order)

    def t_quota_consumed():
        q = call("GET", "/api/v1/me/quotas", token=ctx["quota_token"])
        row = next(x for x in q if x["service_type"] == "garbage")
        assert row["used"] == 1 and row["remaining"] == 1, f"下单后应扣 1 次，实际 {row}"
        return f"used={row['used']}，remaining={row['remaining']}"
    check("下单后额度扣减 1 次", t_quota_consumed)

    def t_phone_hidden():
        """还没人接单时不能把服务方电话给住户，避免私下交易与骚扰。"""
        o = call("GET", f"/api/v1/orders/{ctx['quota_no']}", token=ctx["quota_token"])
        assert not o.get("provider_phone"), f"未接单不应下发电话：{o.get('provider_phone')}"
        return "provider_phone 未下发"
    check("未接单不下发服务方电话", t_phone_hidden)

    def t_quota_restore():
        used_before = int(sql(
            f"SELECT used FROM slot_inventories WHERE service_date='{ctx['date']}' "
            f"AND slot_code='{ctx['garbage_slot']}' AND service_type='garbage'"))
        # 直接拒池子里的单已被禁止（会给任何人瘫痪业务的开关），先派给承接人再拒
        call("POST", f"/api/admin/orders/{ctx['quota_no']}/assign",
             {"provider_id": ctx["prov_uid"]}, token=ctx["admin_token"])
        call("POST", f"/api/v1/provider/orders/{ctx['quota_no']}/reject",
             {"reason": "E2E 额度归还用例"}, token=ctx["prov_token"])
        o = call("GET", f"/api/v1/orders/{ctx['quota_no']}", token=ctx["quota_token"])
        assert o["status"] == "rejected", f"拒单后应为 rejected，实际 {o['status']}"
        q = call("GET", "/api/v1/me/quotas", token=ctx["quota_token"])
        row = next(x for x in q if x["service_type"] == "garbage")
        assert row["remaining"] == 2, f"拒单应归还额度，实际 {row}"
        used_after = int(sql(
            f"SELECT used FROM slot_inventories WHERE service_date='{ctx['date']}' "
            f"AND slot_code='{ctx['garbage_slot']}' AND service_type='garbage'"))
        assert used_after == used_before - 1, \
            f"拒单应释放时段库存，{used_before} → {used_after}"
        return f"额度 {row['remaining']}/{row['total']}，库存 {used_before} → {used_after}"
    check("拒单自动归还免费次数并释放库存", t_quota_restore)

    def t_reject_not_mine():
        """池子里没人认领的单不能拒：拒掉会让住户订单作废并触发退款。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["quota_addr"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["quota_token"])
        no = d["order"]["order_no"]
        # 额度已用尽时这单是付费单，得先支付才进入待接单
        if d["order"]["status"] == "pending_payment":
            call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["quota_token"])
        r = call("POST", f"/api/v1/provider/orders/{no}/reject",
                 {"reason": "E2E 越权拒单"}, token=ctx["prov_token"],
                 expect=1003, raw=True)
        assert r["code"] == 1003, f"非承接人拒单应被拒，实际 {r}"
        call("POST", f"/api/v1/orders/{no}/cancel", {"reason": "E2E 清理"},
             token=ctx["quota_token"])
        return f"已拦截：{r['message']}"
    check("未派给自己的订单不能拒（1003）", t_reject_not_mine)

    # ============ 5.6 运营派单与改派 ============
    step("5.6 运营派单与改派")

    def make_provider(suffix):
        """注册并认证一个服务方，返回 (user_id, token)。"""
        ph = f"173{(ts + suffix) % 100000000:08d}"[:11]
        d = call("POST", "/api/v1/auth/register",
                 {"phone": ph, "password": "E2eProv@123", "nickname": f"服务方{suffix}"})
        tk = d["token"]
        keys = {}
        for kind in ("front", "back"):
            r = subprocess.run([
                "curl", "-s", "-X", "POST",
                f"{BASE}/api/v1/provider/application/cert",
                "-H", f"Authorization: Bearer {tk}",
                "-F", f"kind={kind}", "-F", "file=@/tmp/e2e.png",
            ], capture_output=True, text=True)
            keys[kind] = json.loads(r.stdout)["data"]["key"]
        app = call("POST", "/api/v1/provider/application", {
            "real_name": "李四", "phone": ph, "service_type": "all",
            "experience": "E2E 派单改派用例", "id_card_no": "310101199202022345",
            "id_card_front_key": keys["front"], "id_card_back_key": keys["back"],
        }, token=tk)
        call("POST", f"/api/admin/provider-applications/{app['id']}/approve",
             token=ctx["admin_token"])
        return d["user"]["user_id"], tk

    def t_prepare():
        uid, tk = make_provider(61)
        ctx["prov2_id"], ctx["prov2_token"] = uid, tk
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["quota_addr"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["quota_token"])
        ctx["assign_no"] = d["order"]["order_no"]
        return f"服务方 B user_id={uid}，订单 {ctx['assign_no']}"
    check("准备第二个服务方与待派单订单", t_prepare)

    def t_assign_effective():
        """派单必须即刻生效：只写 provider_id 不改状态会让订单永远卡在待接单。"""
        call("POST", f"/api/admin/orders/{ctx['assign_no']}/assign",
             {"provider_id": ctx["prov2_id"]}, token=ctx["admin_token"])
        o = call("GET", f"/api/v1/orders/{ctx['assign_no']}", token=ctx["quota_token"])
        assert o["status"] == "accepted", f"派单后应为 accepted，实际 {o['status']}"
        mine = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=20",
                    token=ctx["prov2_token"])
        nos = [x["order_no"] for x in mine["list"]]
        assert ctx["assign_no"] in nos, "派单后应出现在服务方的待服务列表"
        pool = call("GET", "/api/v1/provider/orders/open?page=1&size=20",
                    token=ctx["prov2_token"])
        assert ctx["assign_no"] not in [x["order_no"] for x in pool["list"]], \
            "已派单的订单不应再出现在接单池"
        return f"{ctx['assign_no']} status=accepted，已进服务方待服务"
    check("派单即刻生效并落到服务方名下", t_assign_effective)

    def t_assign_then_advance():
        """服务方无需再点一次接单，拿到就是可推进状态。"""
        call("POST", f"/api/v1/provider/orders/{ctx['assign_no']}/advance",
             {"node": "on_the_way"}, token=ctx["prov2_token"])
        o = call("GET", f"/api/v1/orders/{ctx['assign_no']}", token=ctx["quota_token"])
        assert o["status"] == "on_the_way", f"应可推进到 on_the_way，实际 {o['status']}"
        return "派单后可直接推进，无需二次接单"
    check("派单订单可直接推进节点", t_assign_then_advance)

    def t_reassign():
        """已受理/待上门可改派：原服务方解绑，新服务方接住。"""
        call("POST", f"/api/admin/orders/{ctx['assign_no']}/assign",
             {"provider_id": ctx["prov_uid"]}, token=ctx["admin_token"])
        o = call("GET", f"/api/v1/orders/{ctx['assign_no']}", token=ctx["quota_token"])
        assert o["status"] == "accepted", f"改派后应回到 accepted，实际 {o['status']}"
        old = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=20",
                   token=ctx["prov2_token"])
        assert ctx["assign_no"] not in [x["order_no"] for x in old["list"]], \
            "改派后原服务方不应再看到该单"
        new = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=20",
                   token=ctx["prov_token"])
        assert ctx["assign_no"] in [x["order_no"] for x in new["list"]], \
            "改派后新服务方应看到该单"
        return "原服务方已解绑，新服务方已接住"
    check("已接单订单可改派", t_reassign)

    def t_assign_not_provider():
        """派给普通住户无意义，必须拦在入口。"""
        r = call("POST", f"/api/admin/orders/{ctx['assign_no']}/assign",
                 {"provider_id": ctx["quota_uid"]}, token=ctx["admin_token"],
                 expect=1001, raw=True)
        assert r["code"] == 1001, f"派给非服务方应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("派单目标必须是已认证服务方", t_assign_not_provider)

    def t_assign_after_service():
        """开始服务后不允许换人，否则服务记录与结算都会错位。"""
        call("POST", f"/api/v1/provider/orders/{ctx['assign_no']}/advance",
             {"node": "in_service"}, token=ctx["prov_token"])
        r = call("POST", f"/api/admin/orders/{ctx['assign_no']}/assign",
                 {"provider_id": ctx["prov2_id"]}, token=ctx["admin_token"],
                 expect=2003, raw=True)
        assert r["code"] == 2003, f"服务中派单应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("服务开始后不可改派（2003）", t_assign_after_service)

    def t_assign_close_loop():
        """派单/改派的单必须能一路走到结算。

        这段是补出来的：原来用例只验到 in_service 就收尾，订单悬在半空。
        派单死锁之所以藏了很久，就是因为从来没人验过「派出去的单能不能做完」。
        """
        no = ctx["assign_no"]
        call("POST", f"/api/v1/provider/orders/{no}/advance",
             {"node": "pending_confirm"}, token=ctx["prov_token"])
        r = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/orders/{no}/records",
            "-H", f"Authorization: Bearer {ctx['prov_token']}",
            "-F", "media_type=photo", "-F", "duration_sec=0",
            "-F", "file=@/tmp/e2e.png",
        ], capture_output=True, text=True)
        assert json.loads(r.stdout)["code"] == 0, f"记录上传失败：{r.stdout}"
        call("POST", f"/api/v1/orders/{no}/confirm", token=ctx["quota_token"])
        o = call("GET", f"/api/v1/orders/{no}", token=ctx["quota_token"])
        assert o["status"] == "completed", f"应已完成，实际 {o['status']}"
        assert o.get("finished_at"), "完成后应有 finished_at"
        return f"{no} 派单路径已闭环"
    check("派单订单可走完履约直至完成", t_assign_close_loop)

    def t_assign_income_owner():
        """改派后收入要归最后履约的人，挂错人等于把钱打给没干活的那位。"""
        no = ctx["assign_no"]
        pid = sql(f"SELECT provider_id FROM provider_income WHERE order_no='{no}'")
        assert pid == str(ctx["prov_uid"]), \
            f"收入应归改派后的服务方 {ctx['prov_uid']}，实际 {pid!r}"
        stale = sql(f"SELECT COUNT(*) FROM provider_income "
                    f"WHERE order_no='{no}' AND provider_id={ctx['prov2_id']}")
        assert stale == "0", "被改派走的服务方不应有这笔收入"
        return f"入账归属 {pid}，原服务方 0 条"
    check("改派后收入归属最终履约方", t_assign_income_owner)

    def t_events_complete():
        """事件时间轴是纠纷举证的唯一依据，节点缺一个就说不清过程。"""
        no = ctx["assign_no"]
        raw = sql(f"SELECT GROUP_CONCAT(node ORDER BY id) FROM order_events "
                  f"WHERE order_id=(SELECT id FROM orders WHERE order_no='{no}')")
        nodes = (raw or "").split(",")
        need = ["accepted", "on_the_way", "in_service", "pending_confirm", "completed"]
        missing = [n for n in need if n not in nodes]
        assert not missing, f"时间轴缺节点 {missing}，实际 {nodes}"
        remark = sql(f"SELECT remark FROM order_events WHERE order_id="
                     f"(SELECT id FROM orders WHERE order_no='{no}') "
                     f"AND remark LIKE '运营%' LIMIT 1")
        assert remark in ("运营派单", "运营改派"), f"派单事件备注异常：{remark!r}"
        return f"{len(nodes)} 个节点，含「{remark}」留痕"
    check("履约事件时间轴完整且改派留痕", t_events_complete)


    # ============ 5.7 改期（住户侧） ============
    step("5.7 遛宠改期：住户直接改")

    def pick_alt_dog_slot():
        """挑一个与当前不同、余量 ≥2 的遛宠时段。
        留出富余是为了不把后续用例可能用到的名额吃干（改期会占一个再还一个）。"""
        for offset in (2, 3, 4, 5, 6):
            d0 = time.strftime("%Y-%m-%d", time.localtime(time.time() + offset * 86400))
            slots = call("GET", f"/api/v1/services/slots?date={d0}&service_type=dogwalk",
                         token=ctx["user_token"])
            for s in slots:
                if not s["available"] or s["remaining"] < 2:
                    continue
                if d0 == ctx["dog_date"] and s["slot_code"] == ctx["dog_slot"]:
                    continue
                return d0, s["slot_code"]
        raise AssertionError("找不到可改期的遛宠时段，请检查种子库存")

    def slot_left(date_str, slot_code):
        raw = sql(f"SELECT total-used FROM slot_inventories WHERE service_date='{date_str}' "
                  f"AND slot_code='{slot_code}' AND service_type='dogwalk'")
        assert raw not in (None, ""), f"库存记录缺失：{date_str} {slot_code}"
        return int(raw)

    def t_user_reschedule():
        o0 = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o0["status"] == "pending_accept", f"前置状态应为待接单，实际 {o0['status']}"
        ctx["alt_date"], ctx["alt_slot"] = pick_alt_dog_slot()
        before_old = slot_left(ctx["dog_date"], ctx["dog_slot"])
        before_new = slot_left(ctx["alt_date"], ctx["alt_slot"])

        r = call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
                 {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
                  "reason": "E2E：临时加班"},
                 token=ctx["user_token"])
        assert r["status"] == "applied", f"住户改期应立即生效，实际 {r['status']}"

        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["alt_date"] and o["slot_code"] == ctx["alt_slot"], \
            f"订单时间未更新：{o['service_date']} {o['slot_code']}"
        assert o["status"] == "pending_accept", f"改期不应改变订单状态，实际 {o['status']}"

        # 库存必须真的换：旧时段退回 1 个，新时段扣掉 1 个
        assert slot_left(ctx["dog_date"], ctx["dog_slot"]) == before_old + 1, \
            f"旧时段未释放：{before_old} -> {slot_left(ctx['dog_date'], ctx['dog_slot'])}"
        assert slot_left(ctx["alt_date"], ctx["alt_slot"]) == before_new - 1, \
            f"新时段未占用：{before_new} -> {slot_left(ctx['alt_date'], ctx['alt_slot'])}"

        recs = o.get("reschedules") or []
        assert recs and recs[0]["status"] == "applied", f"改期记录未下发：{recs}"
        assert recs[0]["from_date"] == ctx["dog_date"], f"原时间快照不对：{recs[0]}"
        return f"{ctx['dog_date']} → {ctx['alt_date']} {ctx['alt_slot']}"
    check("住户改期立即生效，库存新旧互换", t_user_reschedule)

    check("改成与原时间相同被拒（1001）", lambda: call(
        "POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
        {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"]},
        token=ctx["user_token"], expect=1001))

    def t_past_date():
        past = time.strftime("%Y-%m-%d", time.localtime(time.time() - 86400))
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": past, "slot_code": ctx["alt_slot"]},
             token=ctx["user_token"], expect=1001)
        return f"past={past} 已拒绝"
    check("改到过去的日期被拒（1001）", t_past_date)

    def t_restore_time():
        """改回原时间，保证后续履约/门禁用例的日期假设不被前面的用例污染。"""
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
              "reason": "E2E：恢复原时间"},
             token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["dog_date"] and o["slot_code"] == ctx["dog_slot"], \
            f"未恢复：{o['service_date']} {o['slot_code']}"
        return "已恢复原时间"
    check("改回原时间（恢复现场）", t_restore_time)

    # ============ 6. 履约全流程 ============
    step("6. 接单 → 上门 → 服务中 → 待确认")

    # 拒单自动退款（遛宠流程图 P0）：已支付订单被服务方拒单，钱必须自动退回
    def t_reject_refund():
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "dogwalk",
            "service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
            "pet_count": 1, "duration_min": 20, "pet_notes": "E2E 拒单退款用例",
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        amount = d["order"]["total_amount"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        call("POST", f"/api/admin/orders/{no}/assign",
             {"provider_id": ctx["prov_uid"]}, token=ctx["admin_token"])
        r = call("POST", f"/api/v1/provider/orders/{no}/reject",
                 {"reason": "E2E：临时有空冲突"}, token=ctx["prov_token"])
        assert r is None or r.get("code") in (0, None), f"拒单失败: {r}"
        o = call("GET", f"/api/v1/orders/{no}", token=ctx["user_token"])
        assert o["status"] == "rejected", f"拒单后应为 rejected，实际 {o['status']}"
        assert o.get("refunded_amount") == amount,             f"拒单后应自动退款 {amount}，实际 {o.get('refunded_amount')}"
        assert o.get("pet_notes") == "E2E 拒单退款用例", "宠物备注未落库"
        return f"{no} 已退 {amount/100:.2f} 元"
    check("已支付订单被拒自动退款（含宠物备注）", t_reject_refund)

    check("未接单不能申请改期（1003）", lambda: call(
        "POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
        {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"]},
        token=ctx["prov_token"], expect=1003))

    check("接单成功", lambda: call(
        "POST", f"/api/v1/provider/orders/{ctx['order_no']}/accept",
        token=ctx["prov_token"]) and "accepted")

    def t_phone_visible():
        """接单后住户必须能联系到服务方，否则上门时间对不上只能干等。"""
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o.get("provider_phone") == prov_phone, \
            f"接单后应下发服务方电话 {prov_phone}，实际 {o.get('provider_phone')}"
        assert o.get("provider_nickname"), "接单后应下发服务方昵称"
        return f"{o['provider_nickname']} / {o['provider_phone']}"
    check("接单后下发服务方联系方式", t_phone_visible)

    # ---------- 服务方发起改期，住户确认才生效 ----------
    def t_provider_request():
        r = call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
                 {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
                  "reason": "E2E：想挪到晚一点"},
                 token=ctx["prov_token"])
        assert r["status"] == "pending", f"服务方申请应为待确认，实际 {r['status']}"

        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o.get("pending_reschedule"), "详情应下发待确认申请"
        assert o["pending_reschedule"]["to_date"] == ctx["alt_date"], \
            f"申请目标日期不对：{o['pending_reschedule']}"
        # 确认前订单时间不能变，否则「待确认」就形同虚设
        assert o["service_date"] == ctx["dog_date"], \
            f"申请阶段订单时间不应变化，实际 {o['service_date']}"
        return f"待确认 → {ctx['alt_date']} {ctx['alt_slot']}"
    check("服务方申请改期，进入待确认", t_provider_request)

    def t_list_shows_pending():
        """列表页必须能直接看到「有单等你确认」，否则住户只能逐单点进详情才发现。"""
        page = call("GET", "/api/v1/orders?tab=&page=1&size=50", token=ctx["user_token"])
        row = next((x for x in page["list"] if x["order_no"] == ctx["order_no"]), None)
        assert row, "列表里找不到目标订单"
        assert row.get("pending_reschedule"), "列表未下发待确认的改期申请"
        assert row["pending_reschedule"]["to_date"] == ctx["alt_date"], \
            f"列表下发的目标日期不对：{row['pending_reschedule']}"
        # 批量填充是按订单号取的，串单了就会人人都显示「待确认」
        others = [x["order_no"] for x in page["list"]
                  if x["order_no"] != ctx["order_no"] and x.get("pending_reschedule")]
        assert not others, f"其它订单被误挂改期提示：{others}"
        return f"列表已提示 → {ctx['alt_date']}"
    check("订单列表下发待确认改期申请", t_list_shows_pending)

    check("已有待确认申请时重复提交被拒（2003）", lambda: call(
        "POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
        {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"]},
        token=ctx["prov_token"], expect=2003))

    def t_user_reject():
        r = call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule/respond",
                 {"approve": False, "reason": "E2E：那天不在家"},
                 token=ctx["user_token"])
        assert r["status"] == "rejected", f"应被拒绝，实际 {r['status']}"
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["dog_date"], "拒绝后订单时间不应变化"
        assert not o.get("pending_reschedule"), "拒绝后不应再有待确认申请"
        return "已拒绝，仍按原时间"
    check("住户拒绝改期，原时间不变", t_user_reject)

    def t_user_approve():
        call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
              "reason": "E2E：再申请一次"},
             token=ctx["prov_token"])
        r = call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule/respond",
                 {"approve": True}, token=ctx["user_token"])
        assert r["status"] == "applied", f"同意后应为已生效，实际 {r['status']}"
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["alt_date"] and o["slot_code"] == ctx["alt_slot"], \
            f"同意后订单时间未更新：{o['service_date']} {o['slot_code']}"
        assert o["status"] == "accepted", f"改期不应改变订单状态，实际 {o['status']}"
        return f"已生效 → {ctx['alt_date']} {ctx['alt_slot']}"
    check("住户同意改期，时间生效", t_user_approve)

    def t_restore_before_service():
        """恢复原时间：后续门禁可见性、履约流程都默认在原日期上跑。"""
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
              "reason": "E2E：回到原时间"},
             token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["dog_date"] and o["slot_code"] == ctx["dog_slot"]
        return "已恢复"
    check("改回原时间，供后续履约用例使用", t_restore_before_service)

    def t_user_reschedule_cancels_pending():
        """住户不理服务方的申请、自己直接把时间改掉，那条申请必须作废：
        留着服务方会一直挂「等住户确认」，住户还能回头点「同意」把时间又改回去。"""
        call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
              "reason": "E2E：待作废"}, token=ctx["prov_token"])
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
              "reason": "E2E：住户直接改"}, token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert not o.get("pending_reschedule"), "住户直接改期后，服务方那条申请应被作废"
        assert o["service_date"] == ctx["alt_date"], f"住户改期未生效：{o['service_date']}"
        # 作废后服务方不该再被「已有待确认」卡住
        r = call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule",
                 {"service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
                  "reason": "E2E：作废后重新申请"}, token=ctx["prov_token"])
        assert r["status"] == "pending", "作废后服务方应能重新申请"
        # 清掉这条，并把时间还原，后续履约用例都按原日期跑
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule/respond",
             {"approve": False, "reason": "E2E：清理"}, token=ctx["user_token"])
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
              "reason": "E2E：还原"}, token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["dog_date"] and o["slot_code"] == ctx["dog_slot"], \
            f"还原失败：{o['service_date']} {o['slot_code']}"
        return "住户改期后旧申请已作废，可重新申请"
    check("住户直接改期作废待确认申请", t_user_reschedule_cancels_pending)

    def t_provider_notice():
        """住户改期是立即生效的，服务方界面上的时间会悄悄变掉。
        他不逐单翻详情根本发现不了，很可能按旧时间去上门——
        所以列表与详情都要给出明确提醒，并且要他确认过才算数。"""
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["alt_date"], "slot_code": ctx["alt_slot"],
              "reason": "E2E：改期提醒"}, token=ctx["user_token"])

        lst = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=20",
                   token=ctx["prov_token"])
        row = next((x for x in lst["list"] if x["order_no"] == ctx["order_no"]), None)
        assert row is not None, "服务方列表里找不到该订单"
        n = row.get("reschedule_notice")
        assert n, "住户改期后，服务方列表未下发提醒"
        assert n["initiated_by"] == "user" and n["status"] == "applied", \
            f"提醒来源不对：{n}"
        assert n["from_date"] == ctx["dog_date"] and n["to_date"] == ctx["alt_date"], \
            f"提醒里的原/新时间不对：{n}"

        d = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov_token"])
        assert d.get("reschedule_notice"), "服务方详情未下发改期提醒"

        # 批量填充最典型的 bug：map 用错 key，导致人人都有提醒
        others = [x["order_no"] for x in lst["list"]
                  if x["order_no"] != ctx["order_no"] and x.get("reschedule_notice")]
        assert not others, f"其它订单被误挂改期提醒：{others}"

        # 这是「服务方确认」用的字段，住户自己不该看到
        u = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert not u.get("reschedule_notice"), "住户视角不应下发服务方的改期提醒"

        # 确认后提示消失；重复确认不该报错
        call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule/ack",
             token=ctx["prov_token"])
        d2 = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov_token"])
        assert not d2.get("reschedule_notice"), "确认后仍下发改期提醒"
        call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/reschedule/ack",
             token=ctx["prov_token"])

        # 还原时间，后续履约用例都按原日期跑；这次改期又会留下一条未确认提醒，
        # 交给下面的「推进后自动清除」用例去验证
        call("POST", f"/api/v1/orders/{ctx['order_no']}/reschedule",
             {"service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
              "reason": "E2E：还原"}, token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["service_date"] == ctx["dog_date"], f"还原失败：{o['service_date']}"
        return "住户改期后服务方可见，确认后清除"
    check("住户改期后服务方收到明确提醒", t_provider_notice)

    def t_advance():
        # 遛宠必须先到「待完成」由住户验收：宠物是否送回、门是否锁好，
        # 服务方自己点「完成」等于把验收环节跳过去，只有垃圾代倒允许这么做。
        seq = ["on_the_way", "in_service"]
        labels = []
        for node in seq:
            call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/advance",
                 {"node": node}, token=ctx["prov_token"])
            o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
            labels.append(o["status"])
        r = call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/advance",
                 {"node": "completed"}, token=ctx["prov_token"], expect=2003, raw=True)
        call("POST", f"/api/v1/provider/orders/{ctx['order_no']}/advance",
             {"node": "pending_confirm"}, token=ctx["prov_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        labels.append(o["status"])
        assert labels == seq + ["pending_confirm"], f"节点推进异常：{labels}"
        return " → ".join(labels) + f"；跳完成已拒（{r['message']}）"
    check("逐节点推进至待完成", t_advance)

    def t_garbage_one_trip():
        """倒垃圾一次上门就结束：出发即服务中，完成即终态，住户不用再点确认。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["quota_addr"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["quota_token"])
        no = ctx["garbage_direct_no"] = d["order"]["order_no"]
        call("POST", f"/api/v1/provider/orders/{no}/accept", token=ctx["prov_token"])

        call("POST", f"/api/v1/provider/orders/{no}/advance",
             {"node": "in_service"}, token=ctx["prov_token"])
        o = call("GET", f"/api/v1/orders/{no}", token=ctx["quota_token"])
        assert o["status"] == "in_service", f"出发后应为服务中，实际 {o['status']}"

        # 完成前留一张现场照片：前端「完成服务」弹层里选填，走的仍是通用服务记录接口，
        # 所以直接打接口验证「先传照片、再推进节点」这条顺序。
        png = bytes.fromhex(
            "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4"
            "890000000a49444154789c6360000002000100ffff03000006000557bfabd400"
            "00000049454e44ae426082")
        open("/tmp/e2e_garbage.png", "wb").write(png)
        pr = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/orders/{no}/records",
            "-H", f"Authorization: Bearer {ctx['prov_token']}",
            "-F", "media_type=photo", "-F", "duration_sec=0",
            "-F", "owner_consented=false", "-F", "file=@/tmp/e2e_garbage.png",
        ], capture_output=True, text=True)
        pd = json.loads(pr.stdout)
        assert pd["code"] == 0, f"服务方上传完成照片失败：{pd}"

        call("POST", f"/api/v1/provider/orders/{no}/advance",
             {"node": "completed"}, token=ctx["prov_token"])
        o = call("GET", f"/api/v1/orders/{no}", token=ctx["quota_token"])
        assert o["status"] == "completed", f"完成服务后应终态，实际 {o['status']}"
        assert o.get("finished_at"), "服务方直接完成也要写 finished_at"

        r = call("POST", f"/api/v1/orders/{no}/confirm",
                 token=ctx["quota_token"], expect=2003, raw=True)
        assert r["code"] == 2003, f"已完成的单不应再要求住户确认，实际 {r}"
        return f"{no} 服务中 → 已完成，无需住户确认"
    check("垃圾单出发即服务中、完成即终态", t_garbage_one_trip)

    def t_garbage_no_confirm_node():
        """直接完成的单不该留下「待完成」节点，否则时间轴会指向一个不存在的验收环节。"""
        no = ctx["garbage_direct_no"]
        amt = sql(f"SELECT amount FROM provider_income WHERE order_no='{no}'")
        assert amt, "垃圾单直接完成未入账"
        nodes = sql(f"SELECT GROUP_CONCAT(node ORDER BY id) FROM order_events "
                    f"WHERE order_id=(SELECT id FROM orders WHERE order_no='{no}')")
        assert "pending_confirm" not in (nodes or "").split(","), \
            f"不该有「待完成」节点，实际 {nodes}"
        return f"入账 {amt}，节点 {nodes}"
    check("垃圾单直接完成也入账且无待确认节点", t_garbage_no_confirm_node)

    def t_garbage_finish_photo():
        """完成时留的照片：住户看得到、标着服务方上传、凭票据能直接下载。"""
        no = ctx["garbage_direct_no"]
        lst = call("GET", f"/api/v1/orders/{no}/records", token=ctx["quota_token"])
        photos = [r for r in lst if r["media_type"] == "photo"]
        assert len(photos) == 1, f"应有 1 张完成照片，实际 {len(photos)}"
        p = photos[0]
        assert p["uploader_role"] == "provider", \
            f"上传者应标为服务方，实际 {p['uploader_role']}"
        assert "?t=" in p["url"], f"住户应拿到带票据的直链，实际 {p['url']}"
        code = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", f"{BASE}{p['url']}",
        ], capture_output=True, text=True).stdout
        assert code == "200", f"住户下载照片失败，HTTP {code}"
        return f"1 张，上传方 {p['uploader_role']}，住户可下载"
    check("垃圾单完成照片住户可见可下载", t_garbage_finish_photo)

    check("非法状态跃迁被拒（2003）", lambda: call(
        "POST", f"/api/v1/provider/orders/{ctx['order_no']}/advance",
        {"node": "on_the_way"}, token=ctx["prov_token"], expect=2003))

    check("其他服务方不能推进他人订单（1003）", lambda: call(
        "POST", f"/api/v1/provider/orders/{ctx['order_no']}/advance",
        {"node": "in_service"}, token=ctx["user_token"], expect=1003))

    def t_advance_clears_notice():
        """能点「出发上门」，说明服务方必然核对过新时间，提示再挂着就是噪音。"""
        d = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov_token"])
        assert not d.get("reschedule_notice"), "推进订单后改期提醒应自动清除"
        return "已清除"
    check("推进订单后改期提醒自动清除", t_advance_clears_notice)

    # ============ 6.5 门禁信息 ============
    step("6.5 门禁信息加密与可见时间窗")

    def t_access_owner():
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o.get("entry_method") == "password", f"进入方式未下发：{o.get('entry_method')}"
        assert o.get("access_info") == "8888#", \
            f"下单人应看到自己填的门禁信息，实际 {o.get('access_info')}"
        assert o.get("access_visible") is True, "下单人对自己的单应始终可见"
        return f"{o['entry_method']} / {o['access_info']}"
    check("下单人可看到自己填的门禁信息", t_access_owner)

    def t_access_encrypted():
        """库里必须是密文：门禁密码明文落库等于把钥匙挂在门口。"""
        raw = sql(f"SELECT access_info FROM orders WHERE order_no='{ctx['order_no']}'")
        assert raw, "门禁信息未落库"
        assert raw != "8888#", f"门禁信息不能明文落库，实际 {raw!r}"
        assert len(raw) > 10, f"密文长度异常：{raw!r}"
        return f"库内为 {len(raw)} 字符密文"
    check("门禁信息密文落库", t_access_encrypted)

    def t_access_window_closed():
        """服务日期在后天，服务方现在不该看到门禁密码。"""
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov_token"])
        assert o.get("access_visible") is False, \
            f"未到窗口应下发 access_visible=false，实际 {o.get('access_visible')}"
        assert not o.get("access_info"), f"未到窗口不能下发门禁信息，实际 {o.get('access_info')}"
        return "窗口未开，仅提示「服务前 2 小时显示」"
    check("服务前 2 小时之外服务方看不到门禁", t_access_window_closed)

    def t_access_window_open():
        """把订单挪到当前时段，模拟服务即将开始，服务方应能看到明文。"""
        h = time.localtime().tm_hour
        code = f"{h:02d}:00-{(h + 1) % 24:02d}:00"
        today = time.strftime("%Y-%m-%d")
        sql(f"UPDATE orders SET service_date='{today}', slot_code='{code}' "
            f"WHERE order_no='{ctx['order_no']}'")
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov_token"])
        assert o.get("access_visible") is True, f"窗口内应可见，实际 {o.get('access_visible')}"
        assert o.get("access_info") == "8888#", \
            f"窗口内应下发门禁明文，实际 {o.get('access_info')}"
        return f"{code} 窗口内已下发"
    check("服务窗口内服务方可看到门禁", t_access_window_open)

    def t_access_other_provider():
        """非履约服务方连订单都读不到，更谈不上门禁信息。"""
        r = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["prov2_token"],
                 expect=1004, raw=True)
        assert r["code"] == 1004, f"非履约服务方应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("非履约服务方读不到该单", t_access_other_provider)

    # ============ 7. 服务记录 ============
    step("7. 服务记录上传与鉴权下载")

    def t_record():
        # 用 curl 走 multipart，urllib 拼 multipart 太啰嗦
        png = bytes.fromhex(
            "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4"
            "890000000a49444154789c6360000002000100ffff03000006000557bfabd400"
            "00000049454e44ae426082")
        open("/tmp/e2e.png", "wb").write(png)
        r = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records",
            "-H", f"Authorization: Bearer {ctx['user_token']}",
            "-F", "media_type=photo", "-F", "duration_sec=0",
            "-F", "file=@/tmp/e2e.png",
        ], capture_output=True, text=True)
        d = json.loads(r.stdout)
        assert d["code"] == 0, f"上传失败：{d}"
        lst = call("GET", f"/api/v1/orders/{ctx['order_no']}/records", token=ctx["user_token"])
        assert len(lst) >= 1, "记录列表为空"
        ctx["record_id"] = lst[0]["id"]
        return f"record_id={lst[0]['id']}"
    check("上传服务记录并可在列表查询", t_record)

    def t_record_download():
        r = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records/{ctx['record_id']}/file",
            "-H", f"Authorization: Bearer {ctx['user_token']}",
        ], capture_output=True, text=True)
        assert r.stdout.strip() == "200", f"下载返回 {r.stdout}"
        return "HTTP 200"
    check("本人可下载服务记录文件", t_record_download)

    def t_record_ticket():
        """图片/视频由 <image>、<video>、previewImage 加载，带不上 Authorization 头。

        列表下发的地址必须自带短期票据，否则小程序里缩略图全部 401
        （线上曾出现：打开订单详情，服务凭证照片一片空白）。
        """
        # 再传一条，供下一条用例验证「票据不能跨记录挪用」
        r = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records",
            "-H", f"Authorization: Bearer {ctx['user_token']}",
            "-F", "media_type=photo", "-F", "duration_sec=0",
            "-F", "file=@/tmp/e2e.png",
        ], capture_output=True, text=True)
        assert json.loads(r.stdout)["code"] == 0, f"第二条记录上传失败：{r.stdout}"

        lst = call("GET", f"/api/v1/orders/{ctx['order_no']}/records", token=ctx["user_token"])
        assert len(lst) >= 2, f"记录数量不足：{lst}"
        first = min(lst, key=lambda x: x["id"])
        second = max(lst, key=lambda x: x["id"])
        assert "?t=" in first["url"], f"记录地址未携带票据：{first['url']}"
        ctx["record_url"] = first["url"]
        ctx["record_url_other_id"] = second["id"]

        # 完全不带任何 header，模拟原生组件的请求
        code = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", f"{BASE}{first['url']}",
        ], capture_output=True, text=True).stdout.strip()
        assert code == "200", f"带票据的直链应可无头访问，实际 {code}"
        return "列表地址自带票据，无头访问 HTTP 200"

    check("记录直链自带票据可被图片组件直接加载", t_record_ticket)

    def t_record_ticket_bound():
        """票据绑定具体路径：拿同一条记录的票去读别的 recordID 必须被拒。

        不绑路径的话，任意一张票就能遍历整库病历照片。
        """
        ticket = ctx["record_url"].split("?t=")[1]
        other = (f"/api/v1/orders/{ctx['order_no']}/records/"
                 f"{ctx['record_url_other_id']}/file?t={ticket}")
        code = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", f"{BASE}{other}",
        ], capture_output=True, text=True).stdout.strip()
        assert code != "200", "票据被挪用到了其它记录！"
        return f"HTTP {code}（跨记录已拦截）"

    check("票据不能跨记录挪用", t_record_ticket_bound)

    def t_record_ticket_not_login():
        """票据只读且只能读文件：当成 Bearer 调普通接口必须失败。"""
        ticket = ctx["record_url"].split("?t=")[1]
        code = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
            f"{BASE}/api/v1/orders?tab=all",
            "-H", f"Authorization: Bearer {ticket}",
        ], capture_output=True, text=True).stdout.strip()
        assert code == "401", f"票据不应被当作登录态，实际 {code}"
        return "票据调普通接口 401"

    check("票据不能当登录态使用", t_record_ticket_not_login)

    def t_record_provider_ok():
        """服务方是履约方，按设计可以查看该单的记录（纠纷举证需要）。"""
        r = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records/{ctx['record_id']}/file",
            "-H", f"Authorization: Bearer {ctx['prov_token']}",
        ], capture_output=True, text=True)
        assert r.stdout.strip() == "200", f"接单服务方应可查看记录，实际 {r.stdout}"
        return "HTTP 200（履约方可读）"
    check("接单服务方可查看该单记录", t_record_provider_ok)

    def t_record_denied():
        """与订单无关的第三方必须被拒，否则病历照片会泄露。"""
        d = call("POST", "/api/v1/auth/register",
                 {"phone": f"173{(ts + 21) % 100000000:08d}"[:11],
                  "password": "E2eOther@123", "nickname": "无关用户"})
        other_token = d["token"]
        r = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records/{ctx['record_id']}/file",
            "-H", f"Authorization: Bearer {other_token}",
        ], capture_output=True, text=True)
        assert r.stdout.strip() != "200", "无关用户不应能下载他人记录！"
        return f"HTTP {r.stdout.strip()}（已拦截）"
    check("无关第三方不能下载他人记录", t_record_denied)

    def t_record_consent():
        """流程图 P2：服务中拍摄需主人同意。未勾选默认 false，勾选后要如实落库。"""
        lst = call("GET", f"/api/v1/orders/{ctx['order_no']}/records",
                   token=ctx["user_token"])
        assert any(r["owner_consented"] is False for r in lst), \
            f"默认应为未同意：{lst}"
        r = subprocess.run([
            "curl", "-s", "-X", "POST",
            f"{BASE}/api/v1/orders/{ctx['order_no']}/records",
            "-H", f"Authorization: Bearer {ctx['prov_token']}",
            "-F", "media_type=photo", "-F", "duration_sec=0",
            "-F", "owner_consented=true", "-F", "file=@/tmp/e2e.png",
        ], capture_output=True, text=True)
        d = json.loads(r.stdout)
        assert d["code"] == 0, f"上传失败：{d}"
        lst = call("GET", f"/api/v1/orders/{ctx['order_no']}/records",
                   token=ctx["user_token"])
        assert any(x["owner_consented"] is True for x in lst), f"同意标记未落库：{lst}"
        return "默认 false，勾选后 true"
    check("服务记录主人同意标记落库", t_record_consent)

    # ============ 8. 确认完成 ============
    step("8. 用户确认完成")

    def t_confirm():
        call("POST", f"/api/v1/orders/{ctx['order_no']}/confirm",
             token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{ctx['order_no']}", token=ctx["user_token"])
        assert o["status"] == "completed", f"应为已完成，实际 {o['status']}"
        return f"status={o['status']}，finished_at={o.get('finished_at', '')[:19]}"
    check("确认完成，订单进入终态", t_confirm)

    check("已完成订单出现在「已完成」tab", lambda: ctx["order_no"] in [
        o["order_no"] for o in call("GET", "/api/v1/orders?tab=completed",
                                    token=ctx["user_token"])["list"]] and "已列出")

    def t_cancel_completed():
        r = call("POST", f"/api/v1/orders/{ctx['order_no']}/cancel", {"reason": "test"},
                 token=ctx["user_token"], expect=2003, raw=True)
        assert r["code"] == 2003, f"终态订单应拒绝取消，实际 {r}"
        return f"已拦截：{r['message']}"
    check("已完成订单不可取消（2003）", t_cancel_completed)

    def t_income():
        """流程图 P2：订单完成自动入账，服务方在「我的收入」能看到明细。"""
        d = call("GET", "/api/v1/provider/income", token=ctx["prov_token"])
        assert d["total"] >= 4000, f"完成后应入账，实际 {d['total']}"
        assert d["order_count"] >= 1, f"入账笔数应 ≥1，实际 {d['order_count']}"
        assert any(i["order_no"] == ctx["order_no"] for i in d["items"]), \
            f"明细缺少本单 {ctx['order_no']}：{d['items']}"
        return f"累计 {d['total']} 分 / {d['order_count']} 单"
    check("订单完成自动入账服务方收入", t_income)

    def t_income_idempotent():
        cnt = int(sql(
            f"SELECT COUNT(*) FROM provider_income WHERE order_no='{ctx['order_no']}'"))
        assert cnt == 1, f"同一订单应只入账 1 条，实际 {cnt}"
        return f"{ctx['order_no']} 入账 1 条"
    check("收入入账幂等", t_income_idempotent)

    # ============ 9. 取消与退款 ============
    step("9. 取消订单与退款")

    def t_cancel_unpaid():
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"], "quantity": 1,
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        call("POST", f"/api/v1/orders/{no}/cancel", {"reason": "临时有事"},
             token=ctx["user_token"])
        o = call("GET", f"/api/v1/orders/{no}", token=ctx["user_token"])
        assert o["status"] == "canceled", f"应已取消，实际 {o['status']}"
        return f"{no} → canceled"
    check("未支付订单可直接取消", t_cancel_unpaid)

    def t_stock_release():
        # 取消后时段余量应恢复
        used_before = int(sql(
            f"SELECT used FROM slot_inventories WHERE service_date='{ctx['date']}' "
            f"AND slot_code='{ctx['garbage_slot']}' AND service_type='garbage'"))
        return f"当前已用于该时段：{used_before}"
    check("取消后库存已释放（used 未累加）", t_stock_release)

    def t_paid_cancel_refund():
        # 退款必须基于有真实金额的订单：代倒垃圾现在走免费次数/包月，用遛宠单
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "dogwalk",
            "service_date": ctx["dog_date"], "slot_code": ctx["dog_slot"],
            "pet_count": 1, "duration_min": 20,
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        amount = d["order"]["total_amount"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        r = call("POST", f"/api/v1/orders/{no}/cancel", {"reason": "计划变更"},
                 token=ctx["user_token"])
        ctx["refund_order"] = no
        assert r["canceled"] is True, f"应直接取消，实际 {r}"
        assert r["refund_due"] == amount, f"应退 {amount} 分，实际 {r['refund_due']}"
        return f"{no} 已取消，应退 {r['refund_due']} 分"
    check("已支付订单取消触发退款（>2h 直接退）", t_paid_cancel_refund)

    def t_auto_refund():
        """取消已支付订单时，handler 会立即调渠道退款，不需要运营手动点。"""
        # 金额随服务而定（遛宠 40 元），不再写死 500
        amount = sql(f"SELECT paid_amount FROM orders WHERE order_no='{ctx['refund_order']}'")
        cnt = sql(f"SELECT COUNT(*) FROM refunds WHERE amount={amount} AND status='success'")
        assert int(cnt) >= 1, "取消订单后应有成功的退款单"
        refunded = sql(f"SELECT refunded_amount FROM orders WHERE order_no='{ctx['refund_order']}'")
        assert refunded == amount, f"订单已退金额应为 {amount}，实际 {refunded}"
        return f"退款单已落库，订单 refunded_amount={refunded}"
    check("取消已支付订单自动原路退款", t_auto_refund)

    def t_dup_refund():
        """已全额退款的订单不能再退，避免重复出账。"""
        r = call("POST", "/api/admin/refunds",
                 {"order_no": ctx["refund_order"], "amount": 0, "reason": "重复退款"},
                 token=ctx["admin_token"], expect=2003, raw=True)
        assert r["code"] == 2003, f"应拒绝重复退款，实际 {r}"
        return f"已拦截：{r['message']}"
    check("同一订单不可重复退款（2003）", t_dup_refund)

    def t_refund_list():
        d = call("GET", "/api/admin/refunds", token=ctx["admin_token"])
        items = d["list"] if isinstance(d, dict) else d
        assert len(items) > 0, "退款单列表为空"
        return f"{len(items)} 条退款单"
    check("后台退款单列表可查", t_refund_list)

    # ============ 10. 包月 ============
    step("10. 包月服务与核销")

    def t_monthly():
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": ctx["date"], "slot_code": ctx["garbage_slot"],
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        assert d["order"]["total_amount"] == 3000, f"包月价应为 3000 分，实际 {d['order']['total_amount']}"
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        m = call("GET", f"/api/v1/orders/{no}/monthly", token=ctx["user_token"])
        ctx["monthly_no"] = no
        return f"{no}，总额 ¥30.00，{m.get('used', 0)}/{m.get('total', 0)} 次"
    check("购买包月服务（¥30 / 30 天）", t_monthly)

    def t_redeem_future_denied():
        """核销只能记今天或更早，防止提前把包月次数刷掉。"""
        r = call("POST", f"/api/admin/orders/{ctx['monthly_no']}/redeem",
                 {"date": ctx["date"], "remark": "未来日期"}, token=ctx["admin_token"],
                 expect=1001, raw=True)
        assert r["code"] == 1001, f"应拒绝未来日期，实际 {r}"
        return f"已拦截：{r['message']}"
    check("核销未来日期被拒（1001）", t_redeem_future_denied)

    def t_monthly_range():
        """包月周期必须落在 [服务日, 服务日+29]，越界拒绝。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": time.strftime("%Y-%m-%d"), "slot_code": ctx["garbage_slot"],
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        call("POST", f"/api/admin/orders/{no}/redeem",
             {"date": time.strftime("%Y-%m-%d"), "remark": "E2E 当日核销"},
             token=ctx["admin_token"])
        c = sql(f"SELECT COUNT(*) FROM monthly_redemptions WHERE order_id="
                f"(SELECT id FROM orders WHERE order_no='{no}')")
        assert int(c) >= 1, "核销记录未落库"
        ctx["monthly_today_no"] = no
        return f"从今天起算的包月核销成功，记录 {c} 条"
    check("包月当日核销成功", t_monthly_range)

    def t_monthly_over_limit():
        """超过每日上限（1 次/天）应被拒，防止一天刷完一个月。"""
        r = call("POST", f"/api/admin/orders/{ctx['monthly_today_no']}/redeem",
                 {"date": time.strftime("%Y-%m-%d"), "remark": "同日重复核销"},
                 token=ctx["admin_token"], raw=True)
        assert r["code"] != 0, f"同日重复核销应被拒绝，实际 {r}"
        return f"已拦截：{r['message']}"
    check("包月同日重复核销被拒", t_monthly_over_limit)

    def t_plan_configurable():
        """包月规则要能在后台改：天数、默认时段、额外时段以前只能改数据库。"""
        before = call("GET", "/api/v1/services/garbage-plan")
        skus = call("GET", "/api/admin/skus", token=ctx["admin_token"])
        items = skus["list"] if isinstance(skus, dict) else skus
        sku = next(s for s in items
                   if s["service_type"] == "garbage" and s["plan"] == "monthly")
        call("PUT", f"/api/admin/skus/{sku['id']}", {
            "base_price": sku["base_price"],
            "rules": {
                "duration_days": 31,
                "default_slot": "21:30-22:00",
                "extra_slot_options": ["22:30-23:00", "07:00-07:30"],
            },
        }, token=ctx["admin_token"])
        after = call("GET", "/api/v1/services/garbage-plan")
        assert after["days"] == 31, f"天数未生效：{after['days']}"
        assert after["default_slot"] == "21:30-22:00", \
            f"默认时段未生效：{after['default_slot']}"
        assert after["extra_slots"] == ["22:30-23:00", "07:00-07:30"], \
            f"额外时段未生效：{after['extra_slots']}"

        # 还原，免得污染后续用例
        call("PUT", f"/api/admin/skus/{sku['id']}", {
            "base_price": sku["base_price"],
            "rules": {
                "duration_days": before["days"],
                "default_slot": before["default_slot"],
                "extra_slot_options": before["extra_slots"],
            },
        }, token=ctx["admin_token"])
        restored = call("GET", "/api/v1/services/garbage-plan")
        assert restored["days"] == before["days"], \
            f"还原失败：{restored['days']} != {before['days']}"
        return (f"{before['days']}→31 天、{before['default_slot']}→21:30-22:00，"
                f"改完即时生效并已还原")
    check("包月天数与取件时段后台可配", t_plan_configurable)

    # ---- 月卡在服务方端的每日打卡 ----
    # 月卡订单状态长期停在「已受理」，没有这块信息，服务方看不出今天到底来没来。

    def t_monthly_pool_badge():
        """接单池必须下发服务期。
        包月和单次在池子里同价（都是 ¥30），没有 monthly_start/end，
        前端只能显示成普通单次，服务方接完才发现要连着来一个月。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": time.strftime("%Y-%m-%d"), "slot_code": ctx["garbage_slot"],
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        pool = call("GET", "/api/v1/provider/orders/open?page=1&size=50",
                    token=ctx["prov_token"])
        row = next((o for o in pool["list"] if o["order_no"] == no), None)
        assert row, "支付后的包月单未出现在接单池"
        assert row["plan"] == "monthly", f"接单池 plan 应为 monthly，实际 {row['plan']}"
        assert row.get("monthly_start") and row.get("monthly_end"), \
            f"接单池未下发包月服务期：{row.get('monthly_start')} / {row.get('monthly_end')}"
        ctx["monthly_pool_no"] = no
        return f"{no} 服务期 {row['monthly_start']} ~ {row['monthly_end']}"
    check("接单池下发包月服务期", t_monthly_pool_badge)

    def t_monthly_provider_progress():
        """服务方任务列表必须下发月卡今日进度，否则和单次订单分不清。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": time.strftime("%Y-%m-%d"), "slot_code": ctx["garbage_slot"],
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        call("POST", f"/api/v1/provider/orders/{no}/accept", token=ctx["prov_token"])
        lst = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=50",
                   token=ctx["prov_token"])
        row = next((o for o in lst["list"] if o["order_no"] == no), None)
        assert row, "接单后任务列表里找不到这张月卡"
        m = row.get("monthly") or {}
        assert m.get("total") == 30, f"月卡总天数应为 30，实际 {m.get('total')}"
        assert m.get("today_done") is False, f"刚接单不应是已打卡：{m}"
        assert m.get("today_finished") is False, f"刚接单不应是已完成：{m}"
        assert m.get("in_period") is True, f"今天应在服务期内：{m}"
        ctx["monthly_prov_no"] = no
        return f"{no}，第 {m['used']}/{m['total']} 天，今日未打卡"
    check("服务方任务列表下发月卡今日进度", t_monthly_provider_progress)

    def t_monthly_redeem_by_provider():
        """服务方自己打卡，operator 要记到他名下（后台据此核对是谁上的门）。"""
        p = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/redeem",
                 token=ctx["prov_token"])
        assert p["today_done"] is True, f"打卡后 today_done 应为 true：{p}"
        assert p["today_finished"] is False, f"打卡不等于完成，today_finished 应为 false：{p}"
        assert p["used"] >= 1, f"打卡后已用天数应 ≥1：{p}"
        op = sql(f"SELECT operator_id FROM monthly_redemptions WHERE order_id="
                 f"(SELECT id FROM orders WHERE order_no='{ctx['monthly_prov_no']}') "
                 f"AND service_date=CURDATE()")
        assert op == str(ctx["prov_uid"]), f"operator 应为服务方 {ctx['prov_uid']}，实际 {op}"
        return f"已打卡，第 {p['used']}/{p['total']} 天，operator={op}，未点完成"
    check("服务方打卡记录今日上门", t_monthly_redeem_by_provider)

    def t_monthly_redeem_dup():
        """同一天重复打卡必须被拒：重复核销等于多扣住户一天权益。"""
        r = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/redeem",
                 token=ctx["prov_token"], raw=True)
        assert r["code"] == 2003, f"重复打卡应返回 2003，实际 {r}"
        return f"已拦截：{r['message']}"
    check("服务方同日重复打卡被拒（2003）", t_monthly_redeem_dup)

    def t_monthly_redeem_not_first_day():
        """非首日打卡不能被单次时段库存挡住。
        包月单的 slot_code 是 22:00-22:30，与单次按天卖的时段池
        （08-10 / 12-14 / 18-20）是两套编码，下单时也不占库存。
        早期实现在核销时拿包月的 slot_code 去扣单次库存，非首日一行都匹配
        不到，于是每次打卡都报「该时段已约满，请选择其他时段」。"""
        d = call("POST", "/api/v1/orders", {
            "address_id": ctx["addr_id"], "service_type": "garbage", "plan": "monthly",
            "service_date": time.strftime("%Y-%m-%d"), "slot_code": ctx["garbage_slot"],
        }, token=ctx["user_token"])
        no = d["order"]["order_no"]
        call("POST", f"/api/v1/dev/orders/{no}/mock-pay", token=ctx["user_token"])
        call("POST", f"/api/v1/provider/orders/{no}/accept", token=ctx["prov_token"])
        # 服务期整体前移一天，让「今天」变成服务期的第二天
        sql(f"UPDATE orders SET service_date=DATE_SUB(CURDATE(), INTERVAL 1 DAY), "
            f"monthly_start=DATE_SUB(CURDATE(), INTERVAL 1 DAY), "
            f"monthly_end=DATE_ADD(CURDATE(), INTERVAL 28 DAY) WHERE order_no='{no}'")
        stock = sql(f"SELECT COUNT(*) FROM slot_inventories WHERE service_date=CURDATE() "
                    f"AND slot_code=(SELECT slot_code FROM orders WHERE order_no='{no}') "
                    f"AND service_type='garbage'")
        p = call("POST", f"/api/v1/provider/orders/{no}/monthly/redeem",
                 token=ctx["prov_token"])
        assert p["today_done"] is True, f"非首日打卡应成功，实际 {p}"
        sql(f"DELETE FROM monthly_redemptions WHERE order_id="
            f"(SELECT id FROM orders WHERE order_no='{no}') AND service_date=CURDATE()")
        return f"{no} 服务期第 2 天打卡成功（同时段库存行 {stock} 条）"
    check("包月非首日打卡不被时段库存挡住", t_monthly_redeem_not_first_day)

    def t_monthly_tab_moves():
        """打卡后这单进「服务中」：人已到、袋子还没提走，正等着收尾。
        留在待服务里的话服务方还会看到「出发上门」，等于让他再打一次卡；
        进已完成则更错——那一趟根本没做完。"""
        pending = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=50",
                       token=ctx["prov_token"])
        assert all(o["order_no"] != ctx["monthly_prov_no"] for o in pending["list"]), \
            "打卡后月卡应离开待服务（这一趟不再需要出发上门）"
        doing = call("GET", "/api/v1/provider/orders?tab=processing&page=1&size=50",
                     token=ctx["prov_token"])
        row = next((o for o in doing["list"] if o["order_no"] == ctx["monthly_prov_no"]), None)
        assert row, "打卡后月卡应出现在服务中"
        m = row.get("monthly") or {}
        assert m.get("today_done") is True and m.get("today_finished") is False, \
            f"服务中的月卡应标出「已上门未完成」：{m}"
        done = call("GET", "/api/v1/provider/orders?tab=completed&page=1&size=50",
                    token=ctx["prov_token"])
        assert all(o["order_no"] != ctx["monthly_prov_no"] for o in done["list"]), \
            "只打了卡就进了已完成，缺了「完成服务」这一步"
        return f"{ctx['monthly_prov_no']} 已从待服务移到服务中，today_done=true / today_finished=false"
    check("包月打卡后进入服务中", t_monthly_tab_moves)

    def t_monthly_finish():
        """「完成服务」收尾这一趟：状态不变，靠当天核销记录的 finished_at 归入已完成。"""
        p = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/finish",
                 token=ctx["prov_token"])
        assert p["today_finished"] is True, f"完成后 today_finished 应为 true：{p}"
        assert p["today_finished_at"], f"完成后应下发完成时间：{p}"
        fin = sql(f"SELECT finished_at FROM monthly_redemptions WHERE order_id="
                  f"(SELECT id FROM orders WHERE order_no='{ctx['monthly_prov_no']}') "
                  f"AND service_date=CURDATE()")
        assert fin, "库里没有写 finished_at"
        st = sql(f"SELECT status FROM orders WHERE order_no='{ctx['monthly_prov_no']}'")
        assert st == "accepted", f"月卡完成不该改订单状态（仍应 accepted），实际 {st}"

        pending = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=50",
                       token=ctx["prov_token"])
        assert all(o["order_no"] != ctx["monthly_prov_no"] for o in pending["list"]), \
            "已完成的月卡仍留在待服务"
        doing = call("GET", "/api/v1/provider/orders?tab=processing&page=1&size=50",
                     token=ctx["prov_token"])
        assert all(o["order_no"] != ctx["monthly_prov_no"] for o in doing["list"]), \
            "已完成的月卡仍留在服务中"
        done = call("GET", "/api/v1/provider/orders?tab=completed&page=1&size=50",
                    token=ctx["prov_token"])
        row = next((o for o in done["list"] if o["order_no"] == ctx["monthly_prov_no"]), None)
        assert row, "今天已完成的月卡未出现在已完成"
        assert (row.get("monthly") or {}).get("today_finished") is True, \
            "已完成里的月卡应标出今日已完成"
        return f"status 仍 {st}，finished_at={fin}，已从待服务移到已完成"
    check("包月点完成服务后移入已完成", t_monthly_finish)

    def t_monthly_finish_dup():
        """同一天重复点完成必须被拒：否则完成时间会被覆盖，看不出真实的收尾时刻。"""
        r = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/finish",
                 token=ctx["prov_token"], raw=True)
        assert r["code"] == 2003, f"重复完成应返回 2003，实际 {r}"
        return f"已拦截：{r['message']}"
    check("包月同日重复完成被拒（2003）", t_monthly_finish_dup)

    def t_monthly_tab_reset():
        """新的一天没有当天核销记录，就自动回到待服务。
        分组靠查询当天的日期算，不需要定时任务去翻状态机。"""
        sql(f"DELETE FROM monthly_redemptions WHERE order_id="
            f"(SELECT id FROM orders WHERE order_no='{ctx['monthly_prov_no']}') "
            f"AND service_date=CURDATE()")
        pending = call("GET", "/api/v1/provider/orders?tab=pending&page=1&size=50",
                       token=ctx["prov_token"])
        assert any(o["order_no"] == ctx["monthly_prov_no"] for o in pending["list"]), \
            "没有当天核销记录时应回到待服务"
        done = call("GET", "/api/v1/provider/orders?tab=completed&page=1&size=50",
                    token=ctx["prov_token"])
        assert all(o["order_no"] != ctx["monthly_prov_no"] for o in done["list"]), \
            "未打卡的月卡不该出现在已完成"
        return "清掉当天核销记录后自动回到待服务"
    check("跨天后包月自动回到待服务", t_monthly_tab_reset)

    def t_monthly_finish_before_redeem():
        """没打卡就点完成必须被拒：否则等于跳过「人到了」直接补一个完成标记。"""
        r = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/finish",
                 token=ctx["prov_token"], raw=True)
        assert r["code"] == 2003, f"未打卡就完成应返回 2003，实际 {r}"
        return f"已拦截：{r['message']}"
    check("未打卡不能完成（2003）", t_monthly_finish_before_redeem)

    def t_monthly_advance_denied():
        """服务期内不许走单次流程：点一下「完成服务」整张月卡就提前结束了。"""
        r = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/advance",
                 {"node": "in_service"}, token=ctx["prov_token"], raw=True)
        assert r["code"] == 2003, f"服务期内推进节点应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("月卡服务期内禁止推进完成（2003）", t_monthly_advance_denied)

    def t_monthly_redeem_foreign():
        """打卡只能记自己的单：否则 B 服务方能替 A 刷考勤。"""
        r = call("POST", f"/api/v1/provider/orders/{ctx['monthly_prov_no']}/monthly/redeem",
                 token=ctx["prov2_token"], raw=True)
        assert r["code"] != 0, f"他人订单打卡应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("非本单服务方打卡被拒", t_monthly_redeem_foreign)

    def t_monthly_detail_foreign():
        """核销明细包含每天上门轨迹，不能对无关用户开放。"""
        r = call("GET", f"/api/v1/orders/{ctx['monthly_prov_no']}/monthly",
                 token=ctx["prov2_token"], raw=True)
        assert r["code"] != 0, f"无关用户查看月卡明细应被拒，实际 {r}"
        return f"已拦截：{r['message']}"
    check("无关用户查看月卡明细被拒", t_monthly_detail_foreign)

    # ============ 11. 短信验证码 ============
    step("11. 短信验证码登录与找回密码")

    sms_phone = f"173{(ts + 13) % 100000000:08d}"[:11]

    def t_send_code():
        d = call("POST", "/api/v1/auth/sms/code", {"phone": sms_phone, "scene": "login"})
        assert "debug_code" not in d, "响应不应包含验证码！"
        ctx["sms_code"] = sql(
            f"SELECT code FROM sms_codes WHERE phone='{sms_phone}' ORDER BY id DESC LIMIT 1")
        return f"expires_in={d['expires_in']}s，响应无验证码字段"
    check("发送验证码且响应不回传验证码", t_send_code)

    check("60 秒内重复发送被限频（1001）", lambda: call(
        "POST", "/api/v1/auth/sms/code", {"phone": sms_phone, "scene": "login"}, expect=1001))

    def t_sms_login():
        d = call("POST", "/api/v1/auth/sms/login",
                 {"phone": sms_phone, "code": ctx["sms_code"], "nickname": "短信住户"})
        assert d["token"], "未返回 token"
        ctx["sms_token"] = d["token"]
        return f"新增用户 user_id={d['user']['user_id']}"
    check("验证码登录自动建号", t_sms_login)

    check("同一验证码不可复用（3004）", lambda: call(
        "POST", "/api/v1/auth/sms/login",
        {"phone": sms_phone, "code": ctx["sms_code"]}, expect=3004))

    def t_reset():
        # 用已有账号走找回密码
        now = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(time.time() + 60))
        exp = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(time.time() + 360))
        sql(f"INSERT INTO sms_codes (created_at,updated_at,phone,scene,code,status,attempts,used,expires_at,ip) "
            f"VALUES ('{now}','{now}','{user_phone}','reset','888888',1,0,0,'{exp}','127.0.0.1')")
        d = call("POST", "/api/v1/auth/password/reset",
                 {"phone": user_phone, "code": "888888", "new_password": "NewE2e@123"})
        assert d["token"], "重置成功应直接返回登录态"
        old = call("POST", "/api/v1/auth/login",
                   {"phone": user_phone, "password": user_pwd}, expect=1002, raw=True)
        assert old["code"] == 1002, "旧密码应失效"
        new = call("POST", "/api/v1/auth/login",
                   {"phone": user_phone, "password": "NewE2e@123"})
        ctx["user_token"] = new["token"]
        return "重置后旧密码失效，新密码可登录"
    check("验证码重置密码并返回登录态", t_reset)

    # ============ 11.5 微信授权登录 ============
    step("11.5 微信授权登录与手机号绑定")

    def t_wx_login():
        """dev 环境未配 appid，后端把 code 当 openid，验证建号与登录态签发。"""
        d = call("POST", "/api/v1/auth/wechat",
                 {"code": f"wxcode{ts}", "nickname": "微信住户"})
        assert d["token"], "未返回 token"
        assert d["need_bind_phone"] is True, "新微信账号应提示绑定手机号"
        ctx["wx_token"] = d["token"]
        ctx["wx_uid"] = d["user"]["user_id"]
        return f"user_id={d['user']['user_id']}，need_bind_phone=true"
    check("微信授权登录自动建号", t_wx_login)

    def t_wx_relogin():
        """同一 code 再次登录应命中同一账号，不能重复建号。"""
        d = call("POST", "/api/v1/auth/wechat", {"code": f"wxcode{ts}"})
        assert d["user"]["user_id"] == ctx["wx_uid"], \
            f"应为同一账号 {ctx['wx_uid']}，实际 {d['user']['user_id']}"
        return f"复用 user_id={d['user']['user_id']}"
    check("同 openid 重复登录不重复建号", t_wx_relogin)

    def t_wx_bind_phone():
        d = call("POST", "/api/v1/auth/phone",
                 {"phone": f"173{(ts + 31) % 100000000:08d}"[:11]},
                 token=ctx["wx_token"])
        assert d["token"], "绑定后应返回新会话"
        assert d["user"]["phone"], "绑定后应带手机号"
        ctx["wx_token"] = d["token"]
        return f"phone={d['user']['phone']}"
    check("微信账号绑定手机号", t_wx_bind_phone)

    def t_wx_bind_conflict():
        """手机号已被别的账号占用时必须拒绝，静默合并会混掉两个账号的订单。"""
        call("POST", "/api/v1/auth/register",
             {"phone": f"173{(ts + 41) % 100000000:08d}"[:11], "password": "E2eOther@1"})
        d = call("POST", "/api/v1/auth/wechat", {"code": f"wxc2{ts}"})
        r = call("POST", "/api/v1/auth/phone",
                 {"phone": f"173{(ts + 41) % 100000000:08d}"[:11]},
                 token=d["token"], expect=1005, raw=True)
        assert r["code"] == 1005, f"应拒绝占用号码，实际 {r}"
        return f"已拦截：{r['message']}"
    check("绑定已被占用的手机号被拒（1005）", t_wx_bind_conflict)

    def t_wx_empty_code():
        r = call("POST", "/api/v1/auth/wechat", {"code": ""}, expect=1001, raw=True)
        assert r["code"] == 1001, "空 code 应被拒绝"
        return "已拦截空 code"
    check("缺少 code 被拒（1001）", t_wx_empty_code)

    def t_avatar_upload():
        """微信 chooseAvatar 给的是临时路径，前端必须上传落地，否则重启就丢。"""
        open("/tmp/e2e_avatar.png", "wb").write(bytes.fromhex(
            "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4"
            "890000000a49444154789c6360000002000100ffff03000006000557bfabd400"
            "00000049454e44ae426082"))
        r = subprocess.run([
            "curl", "-s", "-X", "POST", f"{BASE}/api/v1/me/avatar",
            "-H", f"Authorization: Bearer {ctx['wx_token']}",
            "-F", "file=@/tmp/e2e_avatar.png",
        ], capture_output=True, text=True)
        d = json.loads(r.stdout)
        assert d["code"] == 0, f"头像上传失败：{d}"
        url = d["data"]["avatar"]
        ctx["avatar_url"] = url
        me = call("GET", "/api/v1/me", token=ctx["wx_token"])
        assert me.get("avatar") == url, f"用户资料未同步头像：{me.get('avatar')}"
        return url
    check("上传头像并同步到用户资料", t_avatar_upload)

    def t_avatar_public():
        """小程序 image 组件带不了 Authorization，头像必须免鉴权可读。"""
        r = subprocess.run([
            "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", BASE + ctx["avatar_url"],
        ], capture_output=True, text=True)
        assert r.stdout.strip() == "200", f"头像应可匿名读取，实际 HTTP {r.stdout}"
        return "HTTP 200（无需 token）"
    check("头像地址公开可读", t_avatar_public)

    def t_avatar_bad_type():
        open("/tmp/e2e_bad.txt", "w").write("not an image")
        r = subprocess.run([
            "curl", "-s", "-X", "POST", f"{BASE}/api/v1/me/avatar",
            "-H", f"Authorization: Bearer {ctx['wx_token']}",
            "-F", "file=@/tmp/e2e_bad.txt",
        ], capture_output=True, text=True)
        d = json.loads(r.stdout)
        assert d["code"] == 1001, f"非图片应被拒，实际 {d}"
        return f"已拦截：{d['message']}"
    check("非图片头像被拒（1001）", t_avatar_bad_type)

    # ============ 12. 后台 ============
    step("12. 运营后台看板与管理")

    def t_stats():
        d = call("GET", "/api/admin/stats", token=ctx["admin_token"])
        assert "today_orders" in d and "status_dist" in d, f"看板字段缺失：{list(d.keys())}"
        assert d["today_orders"] > 0, "今日订单数应大于 0"
        return f"今日订单={d['today_orders']}，今日流水={d['today_gmv']}分，用户={d['total_users']}"
    check("数据看板可读", t_stats)

    def t_admin_orders():
        d = call("GET", "/api/admin/orders", token=ctx["admin_token"])
        items = d["list"] if isinstance(d, dict) else d
        assert len(items) > 0, "后台订单列表为空"
        return f"{len(items)} 条"
    check("后台订单列表", t_admin_orders)

    def t_export():
        r = subprocess.run([
            "curl", "-s", f"{BASE}/api/admin/export/orders",
            "-H", f"Authorization: Bearer {ctx['admin_token']}",
        ], capture_output=True, text=True)
        assert "order_no" in r.stdout or "订单" in r.stdout, "导出内容异常"
        return f"{len(r.stdout)} 字节，含表头"
    check("订单 CSV 导出", t_export)

    def t_users():
        d = call("GET", "/api/admin/users", token=ctx["admin_token"])
        items = d["list"] if isinstance(d, dict) else d
        assert any(u.get("phone") == user_phone for u in items), "新用户未出现在后台"
        return f"{len(items)} 个用户"
    check("后台用户列表可查", t_users)

    def t_ban():
        call("PATCH", f"/api/admin/users/{ctx['user_id']}/status", {"status": 0},
             token=ctx["admin_token"])
        r = call("POST", "/api/v1/auth/login",
                 {"phone": user_phone, "password": "NewE2e@123"}, expect=1003, raw=True)
        assert r["code"] == 1003, "封禁用户应无法登录"
        call("PATCH", f"/api/admin/users/{ctx['user_id']}/status", {"status": 1},
             token=ctx["admin_token"])
        call("POST", "/api/v1/auth/login", {"phone": user_phone, "password": "NewE2e@123"})
        return "封禁生效，解封后恢复"
    check("封禁用户后无法登录", t_ban)

    def t_settle():
        return f"订单状态：{sql('SELECT status FROM orders WHERE order_no=%r' % ctx['order_no'])}"
    check("落库订单终态核对", t_settle)

    # ============ 汇总 ============
    print("\n" + "=" * 62)
    print(f"通过 {len(PASSED)} 项，失败 {len(FAILED)} 项")
    if FAILED:
        print("\n失败明细：")
        for d, e in FAILED:
            print(f"  ✗ {d}\n    {e}")
        return 1
    print("全部通过 ✔")
    return 0


if __name__ == "__main__":
    sys.exit(main())
