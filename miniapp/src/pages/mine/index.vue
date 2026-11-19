<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { checkArea, createAddress } from '@/api'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const showAdd = ref(false)
const saving = ref(false)
const checkMsg = ref<{ ok: boolean; text: string } | null>(null)

const form = reactive({
  community: '',
  lane: '',
  house_no: '',
  contact_phone: '',
})

onShow(async () => {
  try {
    await user.ensureLogin()
    await user.loadAddresses()
  } catch (e) {
    console.warn('[mine] 初始化失败', e)
  }
})

function openAdd() {
  form.community = user.currentAddress?.community || ''
  form.lane = ''
  form.house_no = ''
  form.contact_phone = ''
  checkMsg.value = null
  showAdd.value = true
}

function closeAdd() {
  showAdd.value = false
}

/** 输入门牌后即时校验范围，避免用户填完整张表才被拒 */
async function onCheck() {
  if (!form.community || !form.lane || !form.house_no) {
    checkMsg.value = null
    return
  }
  try {
    await checkArea({
      community: form.community,
      lane: form.lane,
      house_no: form.house_no,
    })
    checkMsg.value = { ok: true, text: '该地址在服务范围内' }
  } catch (e) {
    checkMsg.value = { ok: false, text: e instanceof ApiError ? e.message : '校验失败' }
  }
}

async function onSave() {
  if (saving.value) return
  if (!form.community || !form.lane || !form.house_no) {
    uni.showToast({ title: '请填写完整地址', icon: 'none' })
    return
  }
  saving.value = true
  try {
    const addr = await createAddress({
      community: form.community.trim(),
      lane: form.lane.trim(),
      house_no: form.house_no.trim(),
      contact_phone: form.contact_phone.trim(),
    })
    await user.loadAddresses()
    user.setCurrentAddress(addr.id)
    showAdd.value = false
    uni.showToast({ title: '已添加', icon: 'success' })
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
  await user.switchRole(role)
}

const isDev = import.meta.env.DEV


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
          <view class="addr-detail">{{ a.lane }} {{ a.house_no }} {{ a.extra }}</view>
        </view>
        <view v-if="user.currentAddressId === a.id" class="tag">默认</view>
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
      <view v-if="isDev" class="menu-item" @tap="onSwitchRole(user.role === 'user' ? 'provider' : 'user')">
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
        <view class="sheet-title">添加常用地址</view>

        <view class="form-item">
          <text class="form-label">小区</text>
          <input v-model="form.community" class="form-input" placeholder="如：东方冠郡二期" />
        </view>
        <view class="form-item">
          <text class="form-label">路弄</text>
          <input v-model="form.lane" class="form-input" placeholder="如：众安路779弄" @blur="onCheck" />
        </view>
        <view class="form-item">
          <text class="form-label">门牌号</text>
          <input v-model="form.house_no" class="form-input" placeholder="如：45号" @blur="onCheck" />
        </view>
        <view class="form-item">
          <text class="form-label">联系电话</text>
          <input
            v-model="form.contact_phone"
            class="form-input"
            type="number"
            maxlength="11"
            placeholder="选填"
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

.addr-item.active .addr-title {
  color: var(--brand-dark);
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
