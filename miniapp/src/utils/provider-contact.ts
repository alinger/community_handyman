import { orderDetail } from '@/api'
import { ApiError } from '@/utils/request'

/**
 * 联系服务方。
 *
 * 规则（产品要求）：服务方接单前不给联系方式，避免绕过平台催单；
 * 接单后才把服务方电话下发给住户。
 *
 * 列表接口不返回联系方式（否则每条订单都要 join 一次用户表），
 * 所以这里按订单号现拉一次详情 —— 点按钮才发请求，列表滚动不受影响。
 */
export async function contactProvider(orderNo: string) {
  uni.showLoading({ title: '加载中', mask: true })

  let detail
  try {
    detail = await orderDetail(orderNo)
  } catch (e) {
    uni.hideLoading()
    uni.showToast({
      title: e instanceof ApiError ? e.message : '加载失败，请稍后重试',
      icon: 'none',
    })
    return
  }
  uni.hideLoading()

  const phone = detail.provider_phone
  if (!phone) {
    uni.showModal({
      title: '联系服务方',
      content: '服务方接单后，可显示联系方式',
      showCancel: false,
    })
    return
  }

  const who = detail.provider_nickname ? `服务方：${detail.provider_nickname}\n` : ''
  uni.showModal({
    title: '联系服务方',
    content: `${who}联系电话：${phone}`,
    confirmText: '拨打',
    cancelText: '关闭',
    success: (r) => {
      if (!r.confirm) return
      uni.makePhoneCall({
        phoneNumber: phone,
        // 用户取消拨号或设备不支持时不弹错误，属于正常操作
        fail: () => {},
      })
    },
  })
}
