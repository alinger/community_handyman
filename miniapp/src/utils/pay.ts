/**
 * 小程序调起支付。
 * 渠道由后端 pay 接口返回决定：
 * - wechat：用返回的 Params 调 uni.requestPayment（JSAPI 参数已由后端签名）；
 * - sandbox：维持原联调体验（MP 弹提示 / H5 走 dev mock-pay）。
 * 任务列表/详情页/下单页共用这一份，避免多处支付逻辑分叉。
 */
import { payOrder } from '@/api'
import { BASE_URL } from '@/utils/request'

export async function startOrderPay(orderNo: string): Promise<void> {
  const pay = await payOrder(orderNo)
  if (pay.provider === 'wechat' && pay.params) {
    // #ifdef MP-WEIXIN
    await new Promise<void>((resolve, reject) => {
      uni.requestPayment({
        provider: 'wxpay',
        timeStamp: pay.params?.timeStamp ?? '',
        nonceStr: pay.params?.nonceStr ?? '',
        package: pay.params?.package ?? '',
        signType: (pay.params?.signType || 'RSA') as 'RSA',
        paySign: pay.params?.paySign ?? '',
        success: () => resolve(),
        fail: (err) => {
          const msg = err?.errMsg || ''
          // 用户主动取消不算失败：订单仍在待支付，可从详情页补付
          if (msg.includes('cancel')) {
            uni.showToast({ title: '支付未完成，可在订单详情页继续支付', icon: 'none' })
            resolve()
            return
          }
          reject(new Error(msg || '支付失败'))
        },
      })
    })
    return
    // #endif
    // #ifndef MP-WEIXIN
    uni.showToast({ title: '微信支付请在微信小程序内完成', icon: 'none' })
    return
    // #endif
  }

  // #ifdef MP-WEIXIN
  uni.showModal({
    title: '待支付',
    content: '开发环境使用沙箱支付，可在订单详情页完成支付',
    showCancel: false,
  })
  return
  // #endif

  // #ifndef MP-WEIXIN
  await new Promise<void>((resolve, reject) => {
    uni.request({
      // 用统一的 BASE_URL，避免和 request.ts 的回落逻辑不一致
      url: `${BASE_URL}/api/v1/dev/orders/${orderNo}/mock-pay`,
      method: 'POST',
      success: () => resolve(),
      fail: () => reject(new Error('模拟支付失败')),
    })
  })
  // #endif
}
