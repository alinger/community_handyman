/**
 * 展示层格式化工具。
 * 金额在后端一律是「分」，只在展示时转元，避免浮点参与计算。
 */

/** 分转元，保留两位小数并去掉无意义的 .00 */
export function yuan(fen: number): string {
  if (!fen) return '0'
  const v = fen / 100
  return Number.isInteger(v) ? String(v) : v.toFixed(2)
}

/** 分转「¥12.00」 */
export function price(fen: number): string {
  return `¥${(fen / 100).toFixed(2)}`
}

/** 服务类型文案 */
export const SERVICE_LABEL: Record<string, string> = {
  garbage: '代倒垃圾',
  dogwalk: '遛宠',
  accompany: '陪诊',
}

/** 服务类型简短说明 */
export const SERVICE_DESC: Record<string, string> = {
  garbage: '按袋计费，单次 5 元起',
  dogwalk: '20 分钟起，可加时长',
  accompany: '按小时计费，人工报价',
}

/** 订单状态文案 */
export const STATUS_LABEL: Record<string, string> = {
  pending_payment: '待支付',
  pending_accept: '待接单',
  accepted: '已受理',
  on_the_way: '待上门',
  in_service: '服务中',
  pending_confirm: '待完成',
  completed: '已完成',
  canceled: '已取消',
  rejected: '已拒单',
}

/** 金额明细里的符号 */
export function signedPrice(fen: number, minus = false): string {
  return (minus ? '-' : '') + price(fen)
}

/** 把 2026-11-20 转成「11月20日 周五」 */
export function friendlyDate(dateStr: string): string {
  const d = new Date(dateStr.replace(/-/g, '/'))
  if (Number.isNaN(d.getTime())) return dateStr
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
  return `${d.getMonth() + 1}月${d.getDate()}日 ${week}`
}

/** 生成未来 n 天的可选日期（含今天） */
export function nextDays(n: number): { value: string; label: string }[] {
  const out: { value: string; label: string }[] = []
  const today = new Date()
  for (let i = 0; i < n; i++) {
    const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() + i)
    const value = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
    const label = i === 0 ? '今天' : i === 1 ? '明天' : i === 2 ? '后天' : friendlyDate(value).split(' ')[0]
    out.push({ value, label })
  }
  return out
}

function pad(n: number): string {
  return n < 10 ? '0' + n : String(n)
}

/** 服务时刻表 */
export const SLOTS = [
  { code: '08-10', label: '08:00-10:00' },
  { code: '12-14', label: '12:00-14:00' },
  { code: '18-20', label: '18:00-20:00' },
]

/** 遛宠在工作日只开晚间，与后端种子数据保持一致 */
export function availableSlots(dateStr: string): typeof SLOTS {
  const d = new Date(dateStr.replace(/-/g, '/'))
  const weekday = d.getDay() >= 1 && d.getDay() <= 5
  return weekday ? SLOTS.filter((s) => s.code === '18-20') : SLOTS
}
