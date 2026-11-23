/**
 * 版本号单一来源。
 * 发版时只改这里 + src/manifest.json 的 versionName/versionCode，其余页面引用本常量，
 * 避免「我的页写着 v1.0.0、抽屉写着 v1.0.0、清单里是 1.1.0」这种对不上。
 */
export const APP_VERSION = '1.1.0'
