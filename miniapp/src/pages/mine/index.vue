<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { checkArea, createAddress, listCommunities, updateAddress } from '@/api'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const showAdd = ref(false)
const editingId = ref(0) // 0 表示新增，非 0 表示编辑该地址
const saving = ref(false)
const checkMsg = ref<{ ok: boolean; text: string } | null>(null)
const communities = ref<string[]>([])

const form = reactive({
  community: '',
  house_no: '',
  contact_name: '',
  contact_phone: '',
})

onShow(async () => {
  try {
    await user.ensureLogin()
    await Promise.all([user.loadAddresses(), loadCommunities()])
  } catch (e) {
    console.warn('[mine] 初始化失败', e)
  }
})

/** 小区只能从后台配置列表点选 */
async function loadCommunities() {
  try {
    communities.value = await listCommunities()
  } catch (e) {
    console.warn('[mine] 小区列表加载失败', e)
  }
}

function chooseCommunity() {
  if (!communities.value.length) {
    uni.showToast({ title: '暂无可选小区', icon: 'none' })
    return
  }
  uni.showActionSheet({
    itemList: communities.value,
    success: (r) => {
      form.community = communities.value[r.tapIndex]
      onCheck()
    },
  })
}

function openAdd() {
  editingId.value = 0
  form.community = ''
  form.house_no = ''
  form.contact_name = ''
  form.contact_phone = ''
  checkMsg.value = null
  showAdd.value = true
}

/** 编辑已有地址：预填原值，保存时走更新接口 */
function openEdit(id: number) {
  const a = user.addresses.find((x) => x.id === id)
  if (!a) return
  editingId.value = id
  form.community = a.community
  form.house_no = a.house_no
  form.contact_name = a.contact_name
  form.contact_phone = a.contact_phone
  checkMsg.value = null
  showAdd.value = true
}

function closeAdd() {
  showAdd.value = false
}

/** 输入门牌后即时校验范围，避免用户填完整张表才被拒 */
async function onCheck() {
  if (!form.community || !form.house_no) {
    checkMsg.value = null
    return
  }
  try {
    await checkArea({
      community: form.community,
      house_no: form.house_no,
    })
    checkMsg.value = { ok: true, text: '该地址在服务范围内' }
  } catch (e) {
    checkMsg.value = { ok: false, text: e instanceof ApiError ? e.message : '校验失败' }
  }
}

async function onSave() {
  if (saving.value) return
  if (!form.community || !form.house_no) {
    uni.showToast({ title: '请选择小区并填写门牌号', icon: 'none' })
    return
  }
  saving.value = true
  try {
    const data = {
      community: form.community.trim(),
      house_no: form.house_no.trim(),
      contact_name: form.contact_name.trim(),
      contact_phone: form.contact_phone.trim(),
    }
    const addr = editingId.value
      ? await updateAddress(editingId.value, data)
      : await createAddress(data)
    await user.loadAddresses()
    user.setCurrentAddress(addr.id)
    showAdd.value = false
    uni.showToast({ title: editingId.value ? '已更新' : '已添加', icon: 'success' })
  } catch (e) {
    uni.showModal({
      title: '添加失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    saving.value = false
  }
}

function pickAddress(id: number) {
  user.setCurrentAddress(id)
  uni.showToast({ title: '已切换默认地址', icon: 'none' })
}

function goOrders() {
  uni.reLaunch({ url: '/pages/order/list' })
}

/**
 * 开发环境切换演示身份（住户 ↔ 服务方）。
 * 生产环境身份由微信登录决定，这个入口不会渲染。
 */
async function onSwitchRole(role: 'user' | 'provider') {
  if (user.role === role) {
    uni.showToast({ title: '已经是该身份', icon: 'none' })
    return
  }
  try {
    await user.switchRole(role)
  } catch (e) {
    // 切换依赖后端的 /dev/login，该路由只在 dev 注册，
    // 生产环境点到这里会失败，给一句人话而不是抛异常
    uni.showModal({
      title: '无法切换',
      content: '身份切换仅演示环境可用，正式环境由微信登录决定身份',
      showCancel: false,
    })
  }
}


function callService() {
  uni.showModal({
    title: '联系客服',
    content: '客服电话：021-0000 0000（工作日 9:00-18:00）',
    showCancel: false,
  })
}
</script>

<template>
  <view class="page">
    <view class="profile">
      <view class="avatar">住</view>
      <view class="profile-info">
        <view class="nickname">{{ user.nickname || '住户' }}</view>
        <view class="uid">ID {{ user.userId || '-' }}</view>
      </view>
    </view>

    <view class="card">
      <view class="row">
        <view class="section-title">常用地址</view>
        <view class="add-btn" @tap="openAdd">＋ 添加</view>
      </view>

      <view v-if="!user.addresses.length" class="hint-block">
        还没有地址，添加后才能预约服务
      </view>

      <view
        v-for="a in user.addresses"
        :key="a.id"
        class="addr-item"
        :class="{ active: user.currentAddressId === a.id }"
        @tap="pickAddress(a.id)"
      >
        <view class="addr-main">
          <view class="addr-title">{{ a.community }}</view>
          <view class="addr-detail">{{ a.house_no }} {{ a.extra }}</view>
          <view v-if="a.contact_name || a.contact_phone" class="addr-contact">
            {{ a.contact_name }}<text v-if="a.contact_name && a.contact_phone"> · </text>{{ a.contact_phone }}
          </view>
        </view>
        <view class="addr-actions">
          <text class="edit-btn" @tap.stop="openEdit(a.id)">编辑</text>
          <view v-if="user.currentAddressId === a.id" class="tag">默认</view>
        </view>
      </view>
    </view>

    <view class="card menu">
      <view class="menu-item" @tap="goOrders">
        <text>我的订单</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="callService">
        <text>联系客服</text>
        <text class="chev">›</text>
      </view>
      <view class="menu-item" @tap="onSwitchRole(user.role === 'user' ? 'provider' : 'user')">
        <text>切换为{{ user.role === 'user' ? '服务方' : '住户' }}（演示）</text>
        <text class="chev">›</text>
      </view>
    </view>

    <view class="card range-note">
      <view class="section-title">服务范围</view>
      <view class="muted">
        东方冠郡一期、东方冠郡二期、万科清林径、兴欣花苑、同润欣汇名邸、同润新云都会、
        招商象屿星耀翠湾、中粮首创禧瑞祥云、丝庐雅苑一期、丝庐雅苑二期、中洲华庭、吉宝汇锦城一期。
      </view>
    </view>

    <!-- 新增地址弹层 -->
    <view v-if="showAdd" class="mask" @tap="closeAdd">
      <view class="sheet" @tap.stop>
        <view class="sheet-title">{{ editingId ? '编辑常用地址' : '添加常用地址' }}</view>

        <view class="form-item form-select-item" @tap="chooseCommunity">
          <text class="form-label">小区</text>
          <text class="form-select" :class="{ placeholder: !form.community }">
            {{ form.community || '请选择小区' }}
          </text>
          <text class="chev">›</text>
        </view>
        <view class="form-item">
          <text class="form-label">门牌号</text>
          <input v-model="form.house_no" class="form-input" placeholder="如：45号" @blur="onCheck" />
        </view>
        <view class="form-item">
          <text class="form-label">联系人</text>
          <input
            v-model="form.contact_name"
            class="form-input"
            maxlength="16"
            placeholder="如：张阿姨"
          />
        </view>
        <view class="form-item">
          <text class="form-label">联系电话</text>
          <input
            v-model="form.contact_phone"
            class="form-input"
            type="number"
            maxlength="11"
            placeholder="上门服务联系用"
          />
        </view>

        <view v-if="checkMsg" class="check-msg" :class="{ ok: checkMsg.ok }">
          {{ checkMsg.text }}
        </view>

        <view class="sheet-actions">
          <view class="btn-ghost half" @tap="closeAdd">取消</view>
          <view class="btn-primary half" :class="{ 'is-disabled': saving }" @tap="onSave">
            {{ saving ? '保存中…' : '保存' }}
          </view>
        </view>
      </view>
    </view>

    <TabBar current="mine" />
  </view>
</template>

<style scoped>
.page {
  padding-top: 24rpx;
}

.profile {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 24rpx 32rpx 32rpx;
}

.avatar {
  width: 104rpx;
  height: 104rpx;
  border-radius: 50%;
  background: var(--brand-light);
  color: var(--brand-dark);
  font-size: 40rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nickname {
  font-size: 34rpx;
  font-weight: 600;
}

.uid {
  font-size: 23rpx;
  color: var(--text-light);
  margin-top: 6rpx;
}

.add-btn {
  font-size: 25rpx;
  color: var(--brand-dark);
  padding: 8rpx 20rpx;
  background: var(--brand-light);
  border-radius: 24rpx;
}

.hint-block {
  font-size: 25rpx;
  color: var(--text-light);
  padding: 24rpx 0;
}

.addr-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.addr-item:last-child {
  border-bottom: none;
}

.addr-title {
  font-size: 28rpx;
  font-weight: 500;
}

.addr-detail {
  font-size: 24rpx;
  color: var(--text-sub);
  margin-top: 6rpx;
}

.addr-contact {
  font-size: 23rpx;
  color: var(--brand-dark);
  margin-top: 6rpx;
}

.addr-item.active .addr-title {
  color: var(--brand-dark);
}

.addr-actions {
  display: flex;
  align-items: center;
  gap: 14rpx;
  flex-shrink: 0;
}

.edit-btn {
  font-size: 23rpx;
  color: var(--brand-dark);
  padding: 6rpx 14rpx;
  border: 1rpx solid var(--brand-light);
  border-radius: 18rpx;
}

.tag {
  font-size: 21rpx;
  color: var(--brand-dark);
  background: var(--brand-light);
  padding: 4rpx 14rpx;
  border-radius: 8rpx;
}

.menu {
  padding: 0 32rpx;
}

.menu-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 28rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
  font-size: 28rpx;
}

.menu-item:last-child {
  border-bottom: none;
}

.chev {
  color: var(--text-light);
}

.range-note {
  line-height: 1.7;
}

.mask {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: 200;
  display: flex;
  align-items: flex-end;
}

.sheet {
  width: 100%;
  background: #fff;
  border-radius: 32rpx 32rpx 0 0;
  padding: 40rpx 32rpx;
  padding-bottom: calc(40rpx + env(safe-area-inset-bottom));
}

.sheet-title {
  font-size: 32rpx;
  font-weight: 600;
  text-align: center;
  margin-bottom: 24rpx;
}

.form-item {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 22rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.form-label {
  width: 150rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.form-input {
  flex: 1;
  font-size: 28rpx;
}

/* 小区为后台配置点选，不可自由输入 */
.form-select-item {
  position: relative;
}

.form-select {
  flex: 1;
  font-size: 28rpx;
  color: var(--text);
}

.form-select.placeholder {
  color: var(--text-light);
}

.check-msg {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: var(--danger);
}

.check-msg.ok {
  color: var(--brand-dark);
}

.sheet-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 36rpx;
}

.half {
  flex: 1;
}
</style>
