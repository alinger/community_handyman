<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  myProviderApplication,
  submitProviderApplication,
  uploadProviderCert,
  type ProviderApplication,
} from '@/api'
import { useUserStore } from '@/store/user'
import { ensureAuth } from '@/utils/auth'
import { ApiError } from '@/utils/request'

const user = useUserStore()

const loading = ref(true)
const app = ref<ProviderApplication | null>(null)
const submitting = ref(false)

/** 表单是否可编辑：未申请、已驳回可填；待审核与已通过只读 */
const canEdit = computed(() => !app.value || app.value.status === 'rejected')

const form = reactive({
  realName: '',
  phone: '',
  serviceType: 'all',
  experience: '',
  idCardNo: '',
})

// 附件：本地临时路径用于预览，key 用于提交
const cert = reactive({
  frontPath: '',
  frontKey: '',
  backPath: '',
  backKey: '',
  healthPath: '',
  healthKey: '',
})
const uploading = ref<'' | 'front' | 'back' | 'health'>('')

const serviceOptions = [
  { value: 'all', label: '三类都接', desc: '代倒垃圾、遛宠、陪诊' },
  { value: 'garbage', label: '代倒垃圾', desc: '上门取件' },
  { value: 'dogwalk', label: '遛宠', desc: '遛狗、喂食' },
  { value: 'accompany', label: '陪诊', desc: '陪同就医' },
]

const serviceLabel = (v: string) => serviceOptions.find((o) => o.value === v)?.label || v

onShow(async () => {
  if (!ensureAuth()) return
  loading.value = true
  try {
    await user.ensureLogin()
    app.value = await myProviderApplication()
    if (app.value) {
      form.realName = app.value.real_name
      form.phone = app.value.phone
      form.serviceType = app.value.service_type
      form.experience = app.value.experience
      cert.frontKey = app.value.has_front ? 'uploaded' : ''
      cert.backKey = app.value.has_back ? 'uploaded' : ''
      cert.healthKey = app.value.has_health ? 'uploaded' : ''
    } else {
      form.phone = user.phone || ''
    }
  } catch (e) {
    console.warn('[provider-apply] 加载失败', e)
  } finally {
    loading.value = false
  }
})

function chooseImage(kind: 'front' | 'back' | 'health') {
  if (!canEdit.value || uploading.value) return
  uni.chooseImage({
    count: 1,
    sizeType: ['compressed'],
    success: async (res) => {
      const path = res.tempFilePaths?.[0]
      if (!path) return
      uploading.value = kind
      try {
        const { key } = await uploadProviderCert(kind, path)
        if (kind === 'front') {
          cert.frontPath = path
          cert.frontKey = key
        } else if (kind === 'back') {
          cert.backPath = path
          cert.backKey = key
        } else {
          cert.healthPath = path
          cert.healthKey = key
        }
        uni.showToast({ title: '已上传', icon: 'success' })
      } catch (e) {
        uni.showModal({
          title: '上传失败',
          content: e instanceof ApiError ? e.message : '请重试',
          showCancel: false,
        })
      } finally {
        uploading.value = ''
      }
    },
  })
}

function preview(kind: 'front' | 'back' | 'health') {
  const path = kind === 'front' ? cert.frontPath : kind === 'back' ? cert.backPath : cert.healthPath
  if (!path) return
  uni.previewImage({ urls: [path], current: path })
}

async function onSubmit() {
  if (submitting.value || !canEdit.value) return
  if (!form.realName.trim()) {
    uni.showToast({ title: '请填写真实姓名', icon: 'none' })
    return
  }
  if (!/^1[3-9]\d{9}$/.test(form.phone.trim())) {
    uni.showToast({ title: '手机号格式不正确', icon: 'none' })
    return
  }
  if (!form.experience.trim()) {
    uni.showToast({ title: '请填写经验说明', icon: 'none' })
    return
  }
  if (!cert.frontKey || !cert.backKey) {
    uni.showToast({ title: '请上传身份证正反面', icon: 'none' })
    return
  }

  submitting.value = true
  try {
    await submitProviderApplication({
      real_name: form.realName.trim(),
      phone: form.phone.trim(),
      service_type: form.serviceType,
      experience: form.experience.trim(),
      id_card_no: form.idCardNo.trim(),
      // 只在本次重新上传时回传 key，否则后端沿用原图
      id_card_front_key: cert.frontKey === 'uploaded' ? '' : cert.frontKey,
      id_card_back_key: cert.backKey === 'uploaded' ? '' : cert.backKey,
      health_cert_key: cert.healthKey === 'uploaded' ? '' : cert.healthKey,
    })
    uni.showToast({ title: '已提交', icon: 'success' })
    setTimeout(() => uni.navigateBack(), 800)
  } catch (e) {
    uni.showModal({
      title: '提交失败',
      content: e instanceof ApiError ? e.message : '请稍后重试',
      showCancel: false,
    })
  } finally {
    submitting.value = false
  }
}

/** 认证通过后进入服务方端 */
function goProvider() {
  uni.reLaunch({ url: '/pages/provider/tasks' })
}

function goHome() {
  uni.reLaunch({ url: '/pages/index/index' })
}
</script>

<template>
  <view class="page">
    <view v-if="loading" class="loading">加载中…</view>

    <template v-else>
      <!-- 状态卡：已提交/已通过/已驳回 -->
      <view v-if="app" class="card status-card" :class="`st-${app.status}`">
        <view class="st-title">{{ app.status_label }}</view>
        <view v-if="app.status === 'pending'" class="st-hint">
          资料已提交，运营会在 1 个工作日内审核，结果可在本页查看
        </view>
        <view v-else-if="app.status === 'approved'" class="st-hint">
          认证已通过，现在可以切换到服务方身份接单了
        </view>
        <view v-else class="st-hint">
          <text class="reject-reason">{{ app.review_note || '资料不符合要求' }}</text>
          <text class="reject-tip">修改后可重新提交</text>
        </view>
        <view v-if="app.reviewed_at" class="st-time">审核时间：{{ app.reviewed_at.slice(0, 16).replace('T', ' ') }}</view>
        <view v-if="app.status === 'approved'" class="st-actions">
          <view class="btn-primary" @tap="goProvider">进入服务方工作台</view>
        </view>
      </view>

      <!-- 首次进入的说明 -->
      <view v-if="!app" class="card intro-card">
        <view class="intro-title">成为服务方</view>
        <view class="intro-line">认证通过后可承接平台订单，按单结算。</view>
        <view class="intro-list">
          <view class="intro-item"><text class="dot" />实名认证：需要上传身份证正反面</view>
          <view class="intro-item"><text class="dot" />承接范围：可选择接哪几类服务</view>
          <view class="intro-item"><text class="dot" />审核时效：1 个工作日内出结果</view>
        </view>
      </view>

      <!-- 表单 -->
      <view v-if="canEdit" class="card">
        <view class="section-title">基本信息</view>

        <view class="f-item">
          <text class="f-label">真实姓名</text>
          <input v-model="form.realName" class="f-input" maxlength="16" placeholder="与身份证一致" />
        </view>
        <view class="f-item">
          <text class="f-label">联系手机</text>
          <input v-model="form.phone" class="f-input" type="number" maxlength="11" placeholder="服务方与住户联系用" />
        </view>
        <view class="f-item">
          <text class="f-label">身份证号</text>
          <input v-model="form.idCardNo" class="f-input" maxlength="18" placeholder="选填，审核时核对" />
        </view>

        <view class="section-title mt">承接服务类型</view>
        <view class="svc-grid">
          <view
            v-for="o in serviceOptions"
            :key="o.value"
            class="svc-opt"
            :class="{ active: form.serviceType === o.value }"
            @tap="form.serviceType = o.value"
          >
            <view class="svc-name">{{ o.label }}</view>
            <view class="svc-desc">{{ o.desc }}</view>
          </view>
        </view>

        <view class="section-title mt">经验说明</view>
        <textarea
          v-model="form.experience"
          class="textarea"
          maxlength="500"
          placeholder="例如：在小区住了 5 年，熟悉周边三甲医院流程；养狗 8 年，能处理大型犬。"
        />
        <view class="counter">{{ form.experience.length }}/500</view>

        <view class="section-title mt">证件照片</view>
        <view class="cert-grid">
          <view class="cert-slot" @tap="cert.frontPath ? preview('front') : chooseImage('front')">
            <image v-if="cert.frontPath" :src="cert.frontPath" class="cert-img" mode="aspectFill" />
            <view v-else class="cert-empty">
              <text class="cert-plus">＋</text>
              <text class="cert-text">{{ uploading === 'front' ? '上传中…' : '身份证人像面' }}</text>
            </view>
            <view v-if="cert.frontKey === 'uploaded'" class="cert-badge">已上传</view>
          </view>
          <view class="cert-slot" @tap="cert.backPath ? preview('back') : chooseImage('back')">
            <image v-if="cert.backPath" :src="cert.backPath" class="cert-img" mode="aspectFill" />
            <view v-else class="cert-empty">
              <text class="cert-plus">＋</text>
              <text class="cert-text">{{ uploading === 'back' ? '上传中…' : '身份证国徽面' }}</text>
            </view>
            <view v-if="cert.backKey === 'uploaded'" class="cert-badge">已上传</view>
          </view>
        </view>
        <view class="hint">照片仅用于资质审核，加密存储，不会公开展示</view>

        <view class="cert-slot full" @tap="cert.healthPath ? preview('health') : chooseImage('health')">
          <image v-if="cert.healthPath" :src="cert.healthPath" class="cert-img" mode="aspectFill" />
          <view v-else class="cert-empty row-empty">
            <text class="cert-plus">＋</text>
            <text class="cert-text">{{ uploading === 'health' ? '上传中…' : '健康证（选填，陪诊必填）' }}</text>
          </view>
          <view v-if="cert.healthKey === 'uploaded'" class="cert-badge">已上传</view>
        </view>

        <view
          class="btn-primary submit"
          :class="{ 'is-disabled': submitting }"
          @tap="onSubmit"
        >
          {{ submitting ? '提交中…' : app?.status === 'rejected' ? '重新提交审核' : '提交审核' }}
        </view>
      </view>

      <!-- 已通过：只读摘要 -->
      <view v-if="app && app.status === 'approved'" class="card">
        <view class="section-title">认证信息</view>
        <view class="kv"><text class="k">真实姓名</text><text class="v">{{ app.real_name }}</text></view>
        <view class="kv"><text class="k">联系电话</text><text class="v">{{ app.phone }}</text></view>
        <view class="kv"><text class="k">承接类型</text><text class="v">{{ serviceLabel(app.service_type) }}</text></view>
        <view class="kv"><text class="k">经验说明</text><text class="v">{{ app.experience }}</text></view>
        <view class="btn-ghost back-btn" @tap="goHome">返回首页</view>
      </view>
    </template>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 0 60rpx;
}

.loading {
  padding: 120rpx 0;
  text-align: center;
  color: var(--text-light);
  font-size: 26rpx;
}

.status-card {
  padding: 32rpx;
}

.status-card.st-pending {
  background: linear-gradient(135deg, #fef5e6 0%, #fdf0dd 100%);
}

.status-card.st-approved {
  background: linear-gradient(135deg, #e9f5f3 0%, #e4f3f0 100%);
}

.status-card.st-rejected {
  background: #fef2f2;
}

.st-title {
  font-size: 38rpx;
  font-weight: 800;
  color: var(--text);
}

.st-hint {
  margin-top: 14rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  line-height: 1.65;
}

.reject-reason {
  display: block;
  color: var(--danger);
  font-weight: 600;
}

.reject-tip {
  display: block;
  margin-top: 8rpx;
  color: var(--text-light);
  font-size: 23rpx;
}

.st-time {
  margin-top: 12rpx;
  font-size: 22rpx;
  color: var(--text-light);
}

.st-actions {
  margin-top: 26rpx;
}

.intro-card {
  padding: 32rpx;
}

.intro-title {
  font-size: 32rpx;
  font-weight: 700;
  color: var(--brand-dark);
}

.intro-line {
  margin-top: 10rpx;
  font-size: 25rpx;
  color: var(--text-sub);
}

.intro-list {
  margin-top: 20rpx;
}

.intro-item {
  display: flex;
  align-items: center;
  gap: 12rpx;
  font-size: 24rpx;
  color: var(--text-sub);
  line-height: 2;
}

.dot {
  width: 8rpx;
  height: 8rpx;
  border-radius: 50%;
  background: var(--brand);
  flex-shrink: 0;
}

.card {
  padding: 32rpx;
}

.section-title {
  font-size: 29rpx;
  font-weight: 600;
  color: var(--text);
}

.section-title.mt {
  margin-top: 36rpx;
}

.f-item {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 24rpx 0;
  border-bottom: 1rpx solid #f3f4f6;
}

.f-label {
  width: 160rpx;
  font-size: 27rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.f-input {
  flex: 1;
  font-size: 28rpx;
}

.svc-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
  margin-top: 18rpx;
}

.svc-opt {
  border: 2rpx solid #eef1f0;
  border-radius: 14rpx;
  padding: 20rpx 22rpx;
  background: #fafcfb;
}

.svc-opt.active {
  border-color: var(--brand);
  background: var(--brand-tint);
}

.svc-name {
  font-size: 27rpx;
  font-weight: 600;
  color: var(--text);
}

.svc-opt.active .svc-name {
  color: var(--brand-dark);
}

.svc-desc {
  margin-top: 6rpx;
  font-size: 21rpx;
  color: var(--text-light);
}

.textarea {
  width: 100%;
  height: 180rpx;
  margin-top: 18rpx;
  padding: 20rpx;
  box-sizing: border-box;
  border: 1rpx solid #eef1f0;
  border-radius: 14rpx;
  font-size: 26rpx;
  line-height: 1.7;
  background: #fafcfb;
}

.counter {
  text-align: right;
  font-size: 21rpx;
  color: var(--text-light);
  margin-top: 8rpx;
}

.cert-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
  margin-top: 18rpx;
}

.cert-slot {
  position: relative;
  height: 200rpx;
  border: 2rpx dashed #dbe3e1;
  border-radius: 14rpx;
  overflow: hidden;
  background: #fafcfb;
}

.cert-slot.full {
  margin-top: 16rpx;
  height: 140rpx;
  border-width: 2rpx;
}

.cert-img {
  width: 100%;
  height: 100%;
}

.cert-empty {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
}

.row-empty {
  flex-direction: row;
  gap: 12rpx;
}

.cert-plus {
  font-size: 40rpx;
  color: #b9c4c1;
  line-height: 1;
}

.cert-text {
  font-size: 22rpx;
  color: var(--text-light);
}

.cert-badge {
  position: absolute;
  right: 10rpx;
  top: 10rpx;
  background: rgba(15, 127, 120, 0.9);
  color: #fff;
  font-size: 19rpx;
  padding: 4rpx 12rpx;
  border-radius: 8rpx;
}

.hint {
  margin-top: 14rpx;
  font-size: 21rpx;
  color: var(--text-light);
  line-height: 1.6;
}

.submit {
  margin-top: 40rpx;
}

.kv {
  display: flex;
  gap: 20rpx;
  padding: 18rpx 0;
  border-bottom: 1rpx solid #f6f7f8;
  font-size: 26rpx;
}

.kv:last-of-type {
  border-bottom: none;
}

.k {
  width: 150rpx;
  color: var(--text-sub);
  flex-shrink: 0;
}

.v {
  flex: 1;
  color: var(--text);
  line-height: 1.6;
}

.back-btn {
  margin-top: 30rpx;
}
</style>
