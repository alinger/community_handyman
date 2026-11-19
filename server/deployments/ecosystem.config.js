// pm2 进程配置：托管 Go 编译产物。
//
// 为什么用 pm2 管 Go 服务：团队已有 pm2 运维习惯，开机自启、日志轮转、
// 崩溃重启都不用另配 systemd 单元。注意 pm2 只是进程外壳，
// Go 的优雅退出靠 main 里监听的 SIGINT/SIGTERM，kill_timeout 必须留够时间。
// 允许用 APP_CONFIG 覆盖配置文件，便于本地用 pm2 验证而不改动生产配置
const CONFIG_FILE = process.env.APP_CONFIG || 'configs/config.prod.yaml';

module.exports = {
  apps: [
    {
      name: 'community-handyman-api',
      // 相对本文件定位到 server/，避免 pm2 在别的目录启动时找不到二进制
      cwd: __dirname + '/..',
      script: './bin/api',
      args: `-config ${CONFIG_FILE}`,
      exec_mode: 'fork',
      instances: 1,

      autorestart: true,
      max_restarts: 10,
      min_uptime: '10s',
      // 单实例 2C4G 机器上，超过 400M 基本是内存泄漏，重启比拖着好
      max_memory_restart: '400M',

      // Go 收到 SIGINT 后要关连接、停定时任务，默认 1.6 秒不够
      kill_timeout: 15000,
      listen_timeout: 10000,
      wait_ready: false,

      error_file: './logs/api-error.log',
      out_file: './logs/api-out.log',
      merge_logs: true,
      time: true,

      env: {
        NODE_ENV: 'production',
        TZ: 'Asia/Shanghai',
      },
    },
  ],
};
