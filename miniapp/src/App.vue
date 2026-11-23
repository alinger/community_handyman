<script setup lang="ts">
import { onLaunch } from '@dcloudio/uni-app'
import { useUserStore } from '@/store/user'

/** 免登录页面：登录/注册页、静态文档页，以及不涉及个人信息的公开信息页 */
const PUBLIC_PAGES = [
  'pages/auth/login',
  'pages/auth/register',
  'pages/auth/forgot',
  'pages/doc/index',
  // 服务范围是后台配置的公开信息，访客从分享/菜单进入不该被弹回登录页
  'pages/scope/index',
]

/** 访客可直接浏览的页面：未登录启停时不要把它们踢回登录页 */
const GUEST_PAGES = ['pages/index/index', 'pages/mine/index', 'pages/doc/index']

/** 服务方侧页面：已经停在这些页时不要再 reLaunch，否则刷新会打断用户 */
const isProviderPage = (path: string) => path.startsWith('pages/provider/')

onLaunch(async () => {
  const user = useUserStore()

  // onLaunch 阶段 getCurrentPages() 可能还是空数组，取启动参数里的 path 更可靠
  const launchPath = (uni.getLaunchOptionsSync()?.path || '').replace(/^\//, '')
  const isPublic = PUBLIC_PAGES.some((p) => launchPath.startsWith(p))

  // 已有 token：恢复欢迎页所需的地址数据后按「视角」分流。
  //
  // 这里必须用 isProviderView（权限 + 视角）而不是 role === 'provider'：
  // 后者会让已认证服务方每次刷新都被拉回工作台，
  // 即使他上次主动切到了住户视角 —— 视角是用户选择，刷新后要保留。
  if (user.isLoggedIn) {
    try {
      await user.loadAddresses()
      // 三点都不满足才分流：
      //   公开页不打断、已在服务方页不抢跳、视角是住户则留在住户侧
      if (user.isProviderView && !isPublic && !isProviderPage(launchPath)) {
        uni.reLaunch({ url: '/pages/provider/tasks' })
      }
    } catch (e) {
      console.warn('[app] 恢复会话失败', e)
    }
    return
  }

  // 未登录：先给开发环境一次机会（H5 ?as= 参数会走 dev 登录）
  try {
    await user.ensureLogin()
    await user.loadAddresses()
    return
  } catch {
    // 未登录，走下面的访客/登录分流
  }

  // 打开的是公开页：一律不打断。
  if (isPublic) return

  // 访客页（首页/我的/文档）只对「主动选了先逛逛」的用户放行。
  //
  // 首页现在是小程序入口页（pages 数组第一项），冷启动 launchPath 必然是它；
  // 若这里无条件放行，未登录用户会先看到首页、再被页面的 ensureAuth 弹回登录页，
  // 视觉上闪一下。所以在 onLaunch 阶段就把「没登录且没选访客」的人送去登录页。
  const isGuestPage = GUEST_PAGES.some((p) => launchPath.startsWith(p))
  if (isGuestPage && user.isGuest) return

  uni.reLaunch({ url: '/pages/auth/login' })
})
</script>

<style>
/*
 * 设计 token 取自 docs/设计稿：主色深青绿 #0f7f78，
 * 薄荷渐变 #87c9bd → #cbeae3，提示与费用块用暖米黄 #fef5e6。
 * 全局改色只动这里，页面里不写死色值。
 */
page {
  --brand: #0f7f78;
  --brand-dark: #0b6b66;
  --brand-light: #e4f3f0;
  --brand-tint: #f2faf8;
  --mint-1: #87c9bd;
  --mint-2: #cbeae3;
  --text: #1f2328;
  --text-sub: #6b7280;
  --text-light: #9aa3ad;
  --bg: #f8f8f5;
  --card: #ffffff;
  --border: #eceef0;
  --accent: #d97706;
  --accent-soft: #fef5e6;
  --danger: #dc2626;

  background-color: var(--bg);
  color: var(--text);
  font-size: 28rpx;
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Helvetica Neue', sans-serif;
}

.page {
  min-height: 100vh;
  padding-bottom: 150rpx;
  box-sizing: border-box;
}

/* 白卡：设计稿是纯白 + 几乎不可见的阴影，没有粗边框 */
.card {
  background: var(--card);
  border-radius: 20rpx;
  padding: 28rpx 30rpx;
  margin: 0 24rpx 20rpx;
}

.section-title {
  font-size: 30rpx;
  font-weight: 600;
  color: var(--text);
}

.muted {
  color: var(--text-sub);
  font-size: 24rpx;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* 主按钮：深青绿实心、圆角、通栏 */
.btn-primary {
  background: var(--brand);
  color: #fff;
  border-radius: 12rpx;
  height: 92rpx;
  line-height: 92rpx;
  text-align: center;
  font-size: 32rpx;
  font-weight: 500;
}

.btn-primary.is-disabled {
  background: #cfd8d6;
}

/* 次按钮：白底 + 浅灰描边 */
.btn-ghost {
  border: 1rpx solid var(--border);
  color: var(--text);
  border-radius: 12rpx;
  height: 88rpx;
  line-height: 88rpx;
  text-align: center;
  font-size: 28rpx;
  background: #fff;
}

.empty {
  padding: 120rpx 0;
  text-align: center;
  color: var(--text-light);
  font-size: 26rpx;
}
</style>
