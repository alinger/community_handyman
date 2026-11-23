<script setup lang="ts">
/**
 * 带拍照位的服务确认弹层，任务列表与订单详情共用，避免两个入口的文案与照片逻辑分叉。
 *
 * - mode=finish（完成服务，垃圾代倒）：垃圾单点完成就是终态，住户不再确认，服务方这一下
 *   点击是订单里唯一的「到过现场」证据。凭证留成可选项——不强制，赶时间的服务方
 *   点两下就结束；愿意留证的能当场拍，不用先完成、再回列表翻到这张单去补传。
 * - mode=start（开始服务，遛宠）：上门接到宠物必须拍照留证（必填），作为服务开始的凭证；
 *   拍的是住户的宠物，按流程图 P2 要求附带「已征得主人同意」勾选。
 *
 * 两种模式都支持多张：一次可选多张，也可分次追加，提交时逐张上传。
 * 不用 uni.showModal：它只给按钮，放不下拍照位与缩略图。
 */
import { computed, ref, watch } from 'vue'

const props = defineProps<{
  visible: boolean
  /** 提交中（含照片上传）：禁用按钮，防止连点把同一单推两次 */
  loading?: boolean
  /** finish=完成服务（凭证选填）；start=开始服务（遛宠，凭证必填） */
  mode?: 'finish' | 'start'
  /** finish 模式下按服务区分文案：垃圾提袋走 / 遛宠送回宠物 */
  service?: 'garbage' | 'dogwalk'
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', photos: string[], consent: boolean): void
}>()

/** 本地临时路径列表，确认时才逐张上传 */
const photos = ref<string[]>([])
/** 已征得宠物主人同意拍照（仅 start 模式展示） */
const consent = ref(true)

const isStart = computed(() => props.mode === 'start')

/** 最多 9 张：微信单次选择上限就是 9，再多上传也拖慢现场节奏 */
const MAX_PHOTOS = 9

/** 各模式的文案集中在一处，模板不散落三元表达式 */
const copy = computed(() => {
  if (isStart.value) {
    return {
      title: '开始服务',
      sub: '上传上门凭证后开始本次遛宠',
      label: '上门凭证',
      tag: '必填',
      slotText: '拍几张接到宠物的照片（可多选）',
      hint: '凭证会出现在服务记录里，出现争议时有据可查。',
      confirmText: '开始服务',
    }
  }
  if (props.service === 'dogwalk') {
    return {
      title: '完成服务',
      sub: '宠物已送回就点确认，住户稍后验收',
      label: '服务凭证',
      tag: '选填',
      slotText: '拍几张宠物送回或服务完成后的照片（可多选）',
      hint: '不拍照也能完成。留了凭证住户会在「服务记录」里看到，出现争议时有据可查。',
      confirmText: '完成服务',
    }
  }
  return {
    title: '完成服务',
    sub: '垃圾已提走就点确认，住户不需要再确认',
    label: '现场照片',
    tag: '选填',
    slotText: '拍几张门口或袋子已提走的照片（可多选）',
    hint: '不拍照也能完成。留了照片住户会在「服务记录」里看到，出现争议时有据可查。',
    confirmText: '完成服务',
  }
})

// 每次打开都清空，否则上一单的照片会挂到下一单上（列表里连点两单很容易发生）
watch(
  () => props.visible,
  (v) => {
    if (v) {
      photos.value = []
      consent.value = true
    }
  },
)

function choose() {
  if (props.loading) return
  const remain = MAX_PHOTOS - photos.value.length
  if (remain <= 0) {
    uni.showToast({ title: `最多 ${MAX_PHOTOS} 张`, icon: 'none' })
    return
  }
  // #ifdef MP-WEIXIN
  uni.chooseMedia({
    count: remain,
    mediaType: ['image'],
    success: (res) => append(res.tempFiles?.map((f) => f.tempFilePath) || []),
  })
  // #endif
  // #ifndef MP-WEIXIN
  uni.chooseImage({
    count: remain,
    sizeType: ['compressed'],
    success: (res) => {
      // 不同平台类型定义不一致：可能是单串也可能是数组，归一化后再追加
      const paths = res.tempFilePaths
      append(Array.isArray(paths) ? paths : paths ? [paths] : [])
    },
  })
  // #endif
}

/** 分次追加时同样受上限约束，超过的静默截断（入口处已 toast 提示过） */
function append(list: string[]) {
  for (const p of list) {
    if (photos.value.length >= MAX_PHOTOS) break
    photos.value.push(p)
  }
}

function remove(i: number) {
  if (props.loading) return
  photos.value.splice(i, 1)
}

function preview(p: string) {
  if (!photos.value.length) return
  uni.previewImage({ urls: photos.value, current: p })
}

/** 失败重试时由父组件调用：把已上传成功的从待传列表移除，重试只传剩下的 */
function dropUploaded(n: number) {
  photos.value = photos.value.slice(n)
}
defineExpose({ dropUploaded })

function submit() {
  if (props.loading) return
  if (isStart.value && !photos.value.length) {
    uni.showToast({ title: '请先拍照上传凭证', icon: 'none' })
    return
  }
  // 拷贝一份再交出去：上传过程中组件内部列表若被调整，不影响父组件手里的待传清单
  emit('confirm', [...photos.value], consent.value)
}

function close() {
  if (props.loading) return
  emit('close')
}
</script>

<template>
  <view v-if="visible" class="fs-mask" @tap="close">
    <!-- 面板自身阻止冒泡，点内容区不会误关 -->
    <view class="fs-panel" @tap.stop>
      <view class="fs-head">
        <text class="fs-title">{{ copy.title }}</text>
        <text class="fs-sub">{{ copy.sub }}</text>
      </view>

      <view class="fs-block">
        <view class="fs-label-row">
          <text class="fs-label">{{ copy.label }}</text>
          <text class="fs-optional" :class="{ required: isStart }">{{ copy.tag }}</text>
        </view>

        <view v-if="photos.length" class="fs-photo-grid">
          <view v-for="(p, i) in photos" :key="p + i" class="fs-photo-wrap">
            <image class="fs-photo" :src="p" mode="aspectFill" @tap="preview(p)" />
            <view class="fs-remove" @tap.stop="remove(i)">✕</view>
          </view>
          <view v-if="photos.length < MAX_PHOTOS" class="fs-slot fs-slot-add" @tap="choose">
            <text class="fs-slot-icon">＋</text>
            <text class="fs-add-text">添加</text>
          </view>
        </view>
        <view v-else class="fs-slot" @tap="choose">
          <text class="fs-slot-icon">📷</text>
          <text class="fs-slot-text">{{ copy.slotText }}</text>
        </view>

        <text class="fs-hint">{{ copy.hint }}</text>

        <!-- 拍的是住户的宠物：按流程图 P2，上传时随勾选留痕主人是否同意 -->
        <view v-if="isStart" class="fs-consent" @tap="consent = !consent">
          <checkbox class="fs-consent-box" :checked="consent" @tap.stop="consent = !consent" />
          <text>已征得宠物主人同意拍照</text>
        </view>
      </view>

      <view class="fs-actions">
        <view class="btn-ghost fs-btn" @tap="close">再等等</view>
        <view class="btn-primary fs-btn" :class="{ off: loading }" @tap="submit">
          {{
            loading
              ? photos.length
                ? isStart
                  ? '上传并开始…'
                  : '上传并完成…'
                : '提交中…'
              : copy.confirmText
          }}
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.fs-mask {
  position: fixed;
  inset: 0;
  z-index: 999;
  background: rgba(20, 24, 28, 0.45);
  display: flex;
  align-items: flex-end;
}

.fs-panel {
  width: 100%;
  background: var(--card);
  border-radius: 28rpx 28rpx 0 0;
  padding: 36rpx 32rpx calc(32rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
}

.fs-head {
  display: flex;
  flex-direction: column;
  margin-bottom: 30rpx;
}

.fs-title {
  font-size: 36rpx;
  font-weight: 600;
  color: var(--text);
  line-height: 1.4;
}

.fs-sub {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

.fs-block {
  margin-bottom: 30rpx;
}

.fs-label-row {
  display: flex;
  align-items: center;
  margin-bottom: 16rpx;
}

.fs-label {
  font-size: 26rpx;
  font-weight: 500;
  color: var(--text);
}

/* 「选填」单独一个灰标签：不写清楚，服务方会以为漏了照片就不能点完成 */
.fs-optional {
  margin-left: 12rpx;
  padding: 2rpx 12rpx;
  font-size: 20rpx;
  line-height: 1.6;
  color: var(--text-light);
  background: #f2f4f4;
  border-radius: 6rpx;
}

/* 开始凭证是硬性要求：标签用警示色，与「选填」一眼区分 */
.fs-optional.required {
  color: #c2571f;
  background: #fdf1e7;
}

.fs-consent {
  display: flex;
  align-items: center;
  margin-top: 16rpx;
  font-size: 23rpx;
  color: var(--text-sub);
}

.fs-consent-box {
  transform: scale(0.8);
  margin-right: 8rpx;
}

/* 虚线空位：和实心按钮区分开，表达「这里可以放东西，但不放也行」 */
.fs-slot {
  height: 200rpx;
  border: 2rpx dashed var(--border);
  border-radius: 16rpx;
  background: #fafbfb;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

/* 网格里的追加格：正方形小格，与缩略图并排 */
.fs-slot-add {
  width: 200rpx;
  height: 200rpx;
}

.fs-add-text {
  margin-top: 8rpx;
  font-size: 22rpx;
  color: var(--text-sub);
}

.fs-photo-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
}

.fs-slot-icon {
  font-size: 48rpx;
  line-height: 1;
}

.fs-slot-text {
  margin-top: 14rpx;
  font-size: 24rpx;
  color: var(--text-sub);
}

.fs-photo-wrap {
  position: relative;
  width: 200rpx;
  height: 200rpx;
}

.fs-photo {
  width: 200rpx;
  height: 200rpx;
  border-radius: 16rpx;
}

/* 删除角标要压在缩略图外沿，且带描边，避免叠在浅色照片上看不清 */
.fs-remove {
  position: absolute;
  top: -14rpx;
  right: -14rpx;
  width: 44rpx;
  height: 44rpx;
  line-height: 44rpx;
  text-align: center;
  font-size: 24rpx;
  color: #fff;
  background: rgba(20, 24, 28, 0.6);
  border: 2rpx solid #fff;
  border-radius: 50%;
}

.fs-hint {
  display: block;
  margin-top: 16rpx;
  font-size: 23rpx;
  line-height: 1.6;
  color: var(--text-light);
}

.fs-actions {
  display: flex;
}

.fs-btn {
  flex: 1;
}

.fs-btn + .fs-btn {
  margin-left: 20rpx;
  flex: 1.4;
}

/* 提交中降低视觉权重，配合文案表达「已在处理」 */
.btn-primary.off {
  background: #cfd8d6;
}
</style>
