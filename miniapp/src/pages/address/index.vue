<script setup lang="ts">
import { reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { checkArea, createAddress, listCommunities, updateAddress } from '@/api'
import { useUserStore } from '@/store/user'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const showAdd = ref(false)
const editingId = ref(0) // 0 表示新增，非 0 表示编辑该地址
const saving = ref(false)
const settingDefault = ref(0) // 正在设为默认的地址 id，防重复点击
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
    console.warn('[address] 初始化失败', e)
  }
})

async function loadCommunities() {
  try {
    communities.value = await listCommunities()
  } catch (e) {
    console.warn('[address] 小区列表加载失败', e)
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
      title: '保存失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    saving.value = false
  }
}

/** 设为默认：持久化到后端，成功后列表按默认重排 */
async function onSetDefault(id: number) {
  if (settingDefault.value) return
  settingDefault.value = id
  try {
    await user.setDefaultAddress(id)
    uni.showToast({ title: '已设为默认', icon: 'success' })
  } catch (e) {
    uni.showModal({
      title: '设置失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    settingDefault.value = 0
  }
}
</script>

<template>
  <view class="page">
    <view v-if="!user.addresses.length" class="empty">
      还没有地址，点击下方按钮添加后才能预约服务
    </view>

    <view
      v-for="a in user.addresses"
      :key="a.id"
      class="card addr-item"
      :class="{ active: a.is_default }"
    >
      <view class="addr-main">
        <view class="addr-title">
          {{ a.community }}
          <text v-if="a.is_default" class="tag">默认</text>
        </view>
        <view class="addr-detail">{{ a.house_no }} {{ a.extra }}</view>
        <view v-if="a.contact_name || a.contact_phone" class="addr-contact">
          {{ a.contact_name }}<text v-if="a.contact_name && a.contact_phone"> · </text>{{ a.contact_phone }}
        </view>
      </view>
      <view class="addr-actions">
        <text
          v-if="!a.is_default"
          class="action-btn primary"
          @tap="onSetDefault(a.id)"
        >{{ settingDefault === a.id ? '设置中…' : '设置默认' }}</text>
        <text class="action-btn" @tap="openEdit(a.id)">编辑</text>
      </view>
    </view>

    <view class="add-wrap">
      <view class="btn-primary" @tap="openAdd">＋ 新增地址</view>
    </view>

    <!-- 新增 / 编辑地址弹层 -->
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
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 160rpx;
}

.empty {
  padding: 80rpx 40rpx;
  text-align: center;
  font-size: 26rpx;
  color: var(--text-light);
  line-height: 1.7;
}

.addr-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20rpx;
  margin-bottom: 20rpx;
  padding: 28rpx;
}

.addr-item.active {
  border-left: 6rpx solid var(--brand);
}

.addr-main {
  flex: 1;
  min-width: 0;
}

.addr-title {
  font-size: 29rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.addr-detail {
  font-size: 25rpx;
  color: var(--text-sub);
  margin-top: 8rpx;
}

.addr-contact {
  font-size: 23rpx;
  color: var(--brand-dark);
  margin-top: 8rpx;
}

.tag {
  font-size: 20rpx;
  font-weight: 400;
  color: var(--brand-dark);
  background: var(--brand-light);
  padding: 3rpx 12rpx;
  border-radius: 8rpx;
}

.addr-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 14rpx;
  flex-shrink: 0;
}

.action-btn {
  font-size: 23rpx;
  color: var(--brand-dark);
  padding: 8rpx 18rpx;
  border: 1rpx solid var(--brand-light);
  border-radius: 20rpx;
  white-space: nowrap;
}

.action-btn.primary {
  background: var(--brand-dark);
  color: #fff;
  border-color: var(--brand-dark);
}

.add-wrap {
  margin-top: 32rpx;
}

.check-msg {
  margin-top: 16rpx;
  font-size: 24rpx;
  color: var(--danger);
}

.check-msg.ok {
  color: var(--brand-dark);
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

.sheet-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 36rpx;
}

.half {
  flex: 1;
}
</style>
