<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { listServiceAreas, type ServiceAreaScope, type ServiceAreaLane } from '@/api'
import { ApiError } from '@/utils/request'

/**
 * 服务范围页：把「我们在哪些小区、哪些门牌号范围内服务」单独成页。
 *
 * 之前这段信息是「我的」页里一段硬编码文字，既不准（后台改范围要发版）
 * 也不好读（12 个小区挤成一行）。现在改成读后端配置的列表卡片：
 * 一张卡一个小区，卡里列路段与门牌区间。
 *
 * 公开接口，不要求登录 —— 访客下单前最想知道的就是「我这儿送不送」。
 */

const list = ref<ServiceAreaScope[]>([])
const loading = ref(true)
const failed = ref('')
const keyword = ref('')

/** 按关键字过滤小区名与路段名，本地过滤即可（数据量是几十条级别） */
const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return list.value
  return list.value.filter(
    (c) =>
      c.community.toLowerCase().includes(kw) ||
      c.lanes.some((l) => l.lane.toLowerCase().includes(kw)),
  )
})

const laneCount = computed(() => list.value.reduce((sum, c) => sum + c.lanes.length, 0))

onLoad(() => {
  load()
})

async function load() {
  loading.value = true
  failed.value = ''
  try {
    list.value = await listServiceAreas()
  } catch (e) {
    failed.value = e instanceof ApiError ? e.message : '加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

/** 门牌区间文案：0-0 不限；to 为 0 表示「X 号起」；起止相同只显示一个 */
function houseRange(l: ServiceAreaLane) {
  const { house_from: from, house_to: to } = l
  if (!from && !to) return ''
  if (from && !to) return `${from} 号起`
  if (from === to) return `${from} 号`
  return `${from}–${to} 号`
}

/** 单双号文案 */
function parity(l: ServiceAreaLane) {
  if (l.house_parity === 'odd') return '单号'
  if (l.house_parity === 'even') return '双号'
  return ''
}

/** 路段行右侧的门牌说明，空串表示不限门牌 */
function laneScope(l: ServiceAreaLane) {
  return [houseRange(l), parity(l)].filter(Boolean).join(' · ') || '不限门牌'
}

function goAddress() {
  uni.navigateTo({ url: '/pages/address/index' })
}

function copySupport() {
  uni.showModal({
    title: '想让这里也开通？',
    content: '把小区名称发给客服（021-0000 0000），我们会按需求排期开通。',
    showCancel: false,
  })
}
</script>

<template>
  <view class="page">
    <!-- 概要 -->
    <view class="hero">
      <view class="hero-title">服务范围</view>
      <view class="hero-sub">
        目前覆盖 <text class="hero-num">{{ list.length }}</text> 个小区、<text class="hero-num">{{ laneCount }}</text>
        个路段。只在上门可达的范围内接单，超出范围暂时无法下单。
      </view>
      <view class="search">
        <text class="search-icon">🔍</text>
        <input
          v-model="keyword"
          class="search-input"
          placeholder="搜索小区或路段，如「东方冠郡」"
          confirm-type="search"
        />
        <text v-if="keyword" class="search-clear" @tap="keyword = ''">✕</text>
      </view>
    </view>

    <!-- 加载中 -->
    <view v-if="loading" class="state">正在加载服务范围…</view>

    <!-- 失败 -->
    <view v-else-if="failed" class="state">
      <view class="state-text">{{ failed }}</view>
      <view class="state-btn" @tap="load">重试</view>
    </view>

    <!-- 空结果 -->
    <view v-else-if="!filtered.length" class="state">
      <view class="state-text">没有匹配「{{ keyword }}」的小区</view>
      <view class="state-btn" @tap="copySupport">申请开通</view>
    </view>

    <!-- 列表卡片 -->
    <view v-else class="list">
      <view v-for="item in filtered" :key="item.community" class="card scope-card">
        <view class="card-head">
          <view class="community">{{ item.community }}</view>
          <view class="badge">已开通</view>
        </view>
        <view class="lanes">
          <view v-for="lane in item.lanes" :key="lane.lane + lane.house_from + lane.house_to" class="lane-row">
            <text class="lane-name">{{ lane.lane }}</text>
            <text class="lane-scope">{{ laneScope(lane) }}</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 底部说明 -->
    <view class="foot">
      <view class="foot-tip">填写地址时会自动校验是否覆盖，不确定可以先试着填一下</view>
      <view class="foot-actions">
        <view class="btn-ghost" @tap="copySupport">申请开通小区</view>
        <view class="btn-primary" @tap="goAddress">去填写地址</view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 60rpx;
}

.hero {
  padding: 12rpx 8rpx 28rpx;
}

.hero-title {
  font-size: 40rpx;
  font-weight: 800;
  color: var(--text);
}

.hero-sub {
  margin-top: 14rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  line-height: 1.75;
}

.hero-num {
  font-weight: 700;
  color: var(--brand);
}

.search {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin-top: 24rpx;
  padding: 0 24rpx;
  height: 84rpx;
  background: var(--card);
  border-radius: 18rpx;
}

.search-icon {
  font-size: 26rpx;
  opacity: 0.55;
}

.search-input {
  flex: 1;
  font-size: 27rpx;
  color: var(--text);
}

.search-clear {
  font-size: 24rpx;
  color: var(--text-light);
  padding: 0 8rpx;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.scope-card {
  padding: 28rpx 32rpx;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
  padding-bottom: 18rpx;
  border-bottom: 1rpx solid var(--border);
}

.community {
  font-size: 31rpx;
  font-weight: 700;
  color: var(--text);
}

.badge {
  flex-shrink: 0;
  padding: 4rpx 16rpx;
  font-size: 22rpx;
  font-weight: 600;
  color: var(--brand-dark);
  background: var(--brand-light);
  border-radius: 999rpx;
}

.lanes {
  padding-top: 8rpx;
}

.lane-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 20rpx;
  padding: 14rpx 0;
}

.lane-name {
  font-size: 27rpx;
  color: var(--text);
  flex-shrink: 0;
  max-width: 60%;
}

.lane-scope {
  font-size: 24rpx;
  color: var(--text-sub);
  text-align: right;
}

.state {
  padding: 120rpx 0;
  text-align: center;
}

.state-text {
  font-size: 26rpx;
  color: var(--text-sub);
  line-height: 1.7;
}

.state-btn {
  display: inline-block;
  margin-top: 24rpx;
  padding: 16rpx 44rpx;
  font-size: 26rpx;
  color: var(--brand);
  background: var(--brand-light);
  border-radius: 999rpx;
}

.foot {
  margin-top: 40rpx;
}

.foot-tip {
  font-size: 23rpx;
  color: var(--text-light);
  text-align: center;
  line-height: 1.7;
}

.foot-actions {
  display: flex;
  gap: 20rpx;
  margin-top: 28rpx;
}

.btn-ghost,
.btn-primary {
  flex: 1;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 28rpx;
  font-weight: 700;
  border-radius: 18rpx;
}

.btn-ghost {
  color: var(--brand);
  background: var(--brand-light);
}

.btn-primary {
  color: #fff;
  background: var(--brand);
}
</style>
