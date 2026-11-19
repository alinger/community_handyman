<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import TabBar from '@/components/TabBar.vue'
import { useUserStore } from '@/store/user'

const user = useUserStore()
const ready = ref(false)

// 价格只做展示，实际以试算接口为准（后端按 SKU 配置计算，改价不发版）
const services = [
  {
    type: 'garbage',
    name: '代倒垃圾',
    icon: '🗑',
    price: '5',
    unit: '元起',
    desc: '上门代倒',
    tone: 'green',
  },
  {
    type: 'dogwalk',
    name: '遛宠',
    icon: '🐕',
    price: '40',
    unit: '元起',
    desc: '20 分钟起',
    tone: 'amber',
  },
  {
    type: 'accompany',
    name: '陪诊',
    icon: '🏥',
    price: '面议',
    unit: '按小时',
    desc: '全程陪同',
    tone: 'blue',
  },
]

const steps = [
  { no: '1', title: '选服务', desc: '按需选择项目与时段' },
  { no: '2', title: '填信息', desc: '地址、数量、特殊要求' },
  { no: '3', title: '等上门', desc: '服务完成，照片留证' },
]

onShow(async () => {
  try {
    await user.ensureLogin()
    // 服务方不进住户首页，直接去任务列表
    if (user.role === 'provider') {
      uni.reLaunch({ url: '/pages/provider/tasks' })
      return
    }
    await user.loadAddresses()
  } catch (e) {
    console.warn('[index] 加载失败', e)
  } finally {
    ready.value = true
  }
})

function goCreate(type: string) {
  uni.navigateTo({ url: `/pages/order/create?type=${type}` })
}

function goAddress() {
  uni.reLaunch({ url: '/pages/mine/index' })
}
</script>

<template>
  <view class="page">
    <view class="hero">
      <view class="hero-top">
        <view class="community" @tap="goAddress">
          <text class="community-name">{{ user.currentAddress?.community || '选择小区' }}</text>
          <text class="arrow">▾</text>
        </view>
        <text class="hero-sub">社区生活，一键到家</text>
      </view>
      <view v-if="!user.hasAddress" class="hero-tip" @tap="goAddress">
        还没有常用地址，先去添加 ›
      </view>
      <view v-else class="hero-tip">
        {{ user.currentAddress?.lane }} {{ user.currentAddress?.house_no }}
      </view>
    </view>

    <view class="services">
      <view
        v-for="s in services"
        :key="s.type"
        class="service-card"
        :class="`tone-${s.tone}`"
        @tap="goCreate(s.type)"
      >
        <view class="icon">{{ s.icon }}</view>
        <view class="service-main">
          <view class="name">{{ s.name }}</view>
          <view class="desc">{{ s.desc }}</view>
        </view>
        <view class="service-price">
          <text class="num">{{ s.price }}</text>
          <text class="unit">{{ s.unit }}</text>
        </view>
        <text class="chev">›</text>
      </view>
    </view>

    <view class="card steps-card">
      <view class="section-title">三步约上门</view>
      <view class="steps">
        <view v-for="s in steps" :key="s.no" class="step">
          <view class="step-no">{{ s.no }}</view>
          <view class="step-text">
            <view class="step-title">{{ s.title }}</view>
            <view class="step-desc">{{ s.desc }}</view>
          </view>
        </view>
      </view>
    </view>

    <view class="card notice">
      <view class="notice-title">服务范围</view>
      <view class="muted">
        目前已开通东方冠郡、万科清林径等 13 个小区。下单前会自动校验门牌号，范围外会提示。
      </view>
    </view>

    <TabBar current="index" />
  </view>
</template>

<style scoped>
.hero {
  background: linear-gradient(160deg, #16a34a 0%, #15803d 100%);
  padding: 40rpx 32rpx 56rpx;
  border-radius: 0 0 32rpx 32rpx;
  color: #fff;
}

.hero-top {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.community {
  display: flex;
  align-items: center;
  gap: 8rpx;
  font-size: 38rpx;
  font-weight: 600;
}

.arrow {
  font-size: 24rpx;
  opacity: 0.85;
}

.hero-sub {
  font-size: 26rpx;
  opacity: 0.9;
}

.hero-tip {
  margin-top: 28rpx;
  background: rgba(255, 255, 255, 0.16);
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  font-size: 26rpx;
}

.services {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  padding: 0 24rpx;
  margin-top: -24rpx;
}

.service-card {
  display: flex;
  align-items: center;
  gap: 20rpx;
  min-width: 0;
  background: #fff;
  border-radius: 24rpx;
  padding: 24rpx;
  box-shadow: 0 6rpx 24rpx rgba(15, 23, 42, 0.06);
}

.icon {
  width: 88rpx;
  height: 88rpx;
  border-radius: 24rpx;
  font-size: 44rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.tone-green .icon {
  background: #dcfce7;
}

.tone-amber .icon {
  background: #fef3c7;
}

.tone-blue .icon {
  background: #dbeafe;
}

.service-main {
  flex: 1;
  min-width: 0;
}

.name {
  font-size: 30rpx;
  font-weight: 600;
}

.desc {
  font-size: 23rpx;
  color: var(--text-sub);
  margin-top: 6rpx;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.service-price {
  display: flex;
  align-items: baseline;
  flex-shrink: 0;
}

.service-price .num {
  font-size: 34rpx;
  font-weight: 700;
  color: var(--brand);
}

.tone-amber .service-price .num {
  color: #d97706;
}

.tone-blue .service-price .num {
  color: #2563eb;
  font-size: 26rpx;
}

.service-price .unit {
  font-size: 20rpx;
  color: var(--text-light);
  margin-left: 4rpx;
}

.chev {
  color: var(--text-light);
  font-size: 30rpx;
  flex-shrink: 0;
}

.steps-card {
  margin-top: 32rpx;
}

.steps {
  display: flex;
  flex-direction: column;
  gap: 24rpx;
}

.step {
  display: flex;
  align-items: center;
  gap: 20rpx;
}

.step-no {
  width: 48rpx;
  height: 48rpx;
  border-radius: 50%;
  background: var(--brand-light);
  color: var(--brand-dark);
  font-size: 24rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.step-title {
  font-size: 28rpx;
  font-weight: 500;
}

.step-desc {
  font-size: 24rpx;
  color: var(--text-sub);
  margin-top: 4rpx;
}

.notice-title {
  font-size: 28rpx;
  font-weight: 600;
  margin-bottom: 12rpx;
}

.notice {
  line-height: 1.7;
}
</style>
