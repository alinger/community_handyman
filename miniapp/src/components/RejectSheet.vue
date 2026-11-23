<script setup lang="ts">
/**
 * 服务方「无法接单」弹层。
 *
 * 为什么不用 uni.showModal：它在微信端 editable 模式下会把 content 塞进输入框当初始值，
 * 输入区域只有四行、内容被裁切，且按钮和配色完全跟随系统，和主系统风格割裂。
 * 这里改成底部弹层 + 放大输入框，两个入口（任务列表 / 订单详情）共用一份，避免文案分叉。
 */
import { computed, ref, watch } from 'vue'

const props = defineProps<{
  visible: boolean
  /** 提交中：禁用按钮，防止连点重复拒单 */
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', reason: string): void
}>()

/** 常见原因，省去服务方打字；仍可在下方补充说明 */
const QUICK = ['时间冲突', '距离太远', '临时有事', '无法进入小区', '已接满']

const picked = ref('')
const detail = ref('')
const inputRef = ref<{ blur?: () => void }>()

// 每次打开都重置，避免上一单的原因串到下一单（任务列表里连点多个「无法接单」）
watch(
  () => props.visible,
  (v) => {
    if (v) {
      picked.value = ''
      detail.value = ''
    }
  },
)

const count = computed(() => detail.value.length)

/** 快捷原因与补充说明合并成一句话；两者都没填时用兜底文案（后端也有同样的兜底） */
const finalReason = computed(() => {
  const d = detail.value.trim()
  return [picked.value, d].filter(Boolean).join('，')
})

function pick(item: string) {
  picked.value = picked.value === item ? '' : item
}

function close() {
  if (props.loading) return
  inputRef.value?.blur?.()
  emit('close')
}

function submit() {
  if (props.loading) return
  inputRef.value?.blur?.()
  emit('confirm', finalReason.value)
}
</script>

<template>
  <view v-if="visible" class="rs-mask" @tap="close">
    <!-- 面板本身阻止冒泡，点内容区不会误关 -->
    <view class="rs-panel" @tap.stop>
      <view class="rs-head">
        <text class="rs-title">无法接单</text>
        <text class="rs-sub">说明原因后提交，住户会看到</text>
      </view>

      <view class="rs-warn">
        <text class="rs-warn-dot">!</text>
        <text class="rs-warn-text">订单将关闭，已付费用原路退回住户；此单不会回到接单池。</text>
      </view>

      <view class="rs-block">
        <text class="rs-label">常见原因</text>
        <view class="rs-chips">
          <view
            v-for="item in QUICK"
            :key="item"
            class="rs-chip"
            :class="{ on: picked === item }"
            @tap="pick(item)"
          >
            {{ item }}
          </view>
        </view>
      </view>

      <view class="rs-block">
        <view class="rs-label-row">
          <text class="rs-label">补充说明</text>
          <text class="rs-count">{{ count }}/100</text>
        </view>
        <textarea
          ref="inputRef"
          v-model="detail"
          class="rs-input"
          placeholder="选填。例如：家里临时有事，赶不过去"
          placeholder-class="rs-ph"
          :maxlength="100"
          :cursor-spacing="32"
          :adjust-position="true"
        />
      </view>

      <view class="rs-actions">
        <view class="btn-ghost rs-btn" @tap="close">再想想</view>
        <view class="rs-confirm rs-btn" :class="{ off: loading }" @tap="submit">
          {{ loading ? '提交中…' : '确认无法接单' }}
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.rs-mask {
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgba(20, 24, 28, 0.45);
  display: flex;
  align-items: flex-end;
}

/* 底部弹层：顶部大圆角，底部留安全区，键盘弹起时整体上移由 adjust-position 处理 */
.rs-panel {
  width: 100%;
  background: var(--card);
  border-radius: 28rpx 28rpx 0 0;
  padding: 36rpx 32rpx calc(32rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.rs-head {
  display: flex;
  flex-direction: column;
  margin-bottom: 24rpx;
}

.rs-title {
  font-size: 36rpx;
  font-weight: 600;
  color: var(--text);
  line-height: 1.4;
}

.rs-sub {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

/* 后果提示：用浅红底强调不可逆，但不抢主按钮的视觉重量 */
.rs-warn {
  display: flex;
  align-items: flex-start;
  padding: 20rpx 22rpx;
  border-radius: 14rpx;
  background: #fef2f2;
  margin-bottom: 30rpx;
}

.rs-warn-dot {
  flex-shrink: 0;
  width: 30rpx;
  height: 30rpx;
  margin-right: 14rpx;
  border-radius: 50%;
  background: var(--danger);
  color: #fff;
  font-size: 22rpx;
  font-weight: 600;
  line-height: 30rpx;
  text-align: center;
}

.rs-warn-text {
  flex: 1;
  font-size: 25rpx;
  line-height: 1.55;
  color: #b91c1c;
}

.rs-block {
  margin-bottom: 28rpx;
}

.rs-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rs-label {
  font-size: 26rpx;
  font-weight: 500;
  color: var(--text);
}

.rs-count {
  font-size: 22rpx;
  color: var(--text-light);
}

.rs-chips {
  display: flex;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.rs-chip {
  margin: 0 14rpx 14rpx 0;
  padding: 0 24rpx;
  height: 62rpx;
  line-height: 62rpx;
  font-size: 26rpx;
  color: var(--text-sub);
  background: #f6f7f8;
  border: 1rpx solid transparent;
  border-radius: 999rpx;
}

.rs-chip.on {
  color: var(--brand);
  background: var(--brand-light);
  border-color: var(--brand);
  font-weight: 500;
}

/* 输入框放大到 260rpx（约 6 行），避免原生弹窗那种「只能看到半句话」 */
.rs-input {
  width: 100%;
  height: 260rpx;
  margin-top: 16rpx;
  padding: 22rpx 24rpx;
  box-sizing: border-box;
  font-size: 28rpx;
  line-height: 1.6;
  color: var(--text);
  background: #f6f7f8;
  border: 1rpx solid var(--border);
  border-radius: 14rpx;
}

.rs-ph {
  color: var(--text-light);
  font-size: 27rpx;
}

.rs-actions {
  display: flex;
  margin-top: 10rpx;
}

.rs-btn {
  flex: 1;
}

.rs-btn + .rs-btn {
  margin-left: 20rpx;
}

.rs-confirm {
  flex: 1.4;
  background: var(--danger);
  color: #fff;
  border-radius: 12rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 30rpx;
  font-weight: 500;
}

/* 提交中降低视觉权重，配合 loading 文案一起表达「已在处理」 */
.rs-confirm.off {
  background: #f0a3a3;
}
</style>
