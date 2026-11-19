<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'

/**
 * 静态文档页：服务条款 / 隐私政策 / 关于我们。
 * 三类内容结构相同（标题 + 段落列表），用一份页面按 type 渲染，
 * 免得为三页纯文本各建一个文件。
 */
type DocType = 'terms' | 'privacy' | 'about'

interface Section {
  h: string
  p: string[]
}

const DOCS: Record<DocType, { title: string; intro: string; sections: Section[] }> = {
  terms: {
    title: '服务条款',
    intro: '欢迎使用小区好帮手。本条款约定你与平台之间的服务关系，下单即表示你已阅读并同意以下内容。',
    sections: [
      {
        h: '一、服务内容',
        p: [
          '平台提供代倒垃圾、遛宠、陪诊三类社区上门与陪同服务，服务范围限定在已开通的小区与路段内。',
          '陪诊服务仅提供跑腿与信息转述，不出具任何诊疗意见，不承接重症及精神类疾病患者，不垫付任何费用。',
        ],
      },
      {
        h: '二、服务边界',
        p: [
          '代倒垃圾不接收建筑垃圾，须按社区规定自行处理。',
          '遛宠过程中如出现宠物抗拒、应激或有攻击倾向，服务方可终止遛狗，仅保留喂食与换尿垫，订单按部分完成处理并记录原因。',
          '所有服务须合法合规，涉及违法违规的请求平台有权拒绝。',
        ],
      },
      {
        h: '三、取消与退款',
        p: [
          '服务开始前 2 小时以上取消，可免费取消并原路退款。',
          '服务开始前 2 小时内取消，需提交运营审核后再处理。',
          '服务开始后不可取消。',
          '未支付订单 15 分钟后自动关闭并释放时段。',
        ],
      },
      {
        h: '四、包月服务说明',
        p: [
          '包月订单自首次服务日起 30 天为一个周期，含每日一次上门。',
          '当日未使用的次数不累计、不顺延，过期作废。',
        ],
      },
      {
        h: '五、责任约定',
        p: [
          '宠主未如实告知宠物健康状况（如疾病、攻击性）导致服务者受伤的，服务者不承担赔偿责任。',
          '平台不承担宠物医疗责任，不代为判断宠物健康状况。',
          '服务记录（照片、视频）是纠纷处理的凭证，上传后不可删除，仅可追加说明。',
        ],
      },
    ],
  },
  privacy: {
    title: '隐私政策',
    intro: '我们重视你的个人信息与家庭隐私，本政策说明我们收集哪些信息、如何使用以及如何保护。',
    sections: [
      {
        h: '一、我们收集的信息',
        p: [
          '账号信息：昵称、手机号、头像，用于识别账号与服务方联系。',
          '地址信息：小区、门牌号、联系人、联系电话，仅用于派单与上门服务。',
          '订单信息：服务类型、时间、金额、服务记录，用于履约与售后。',
        ],
      },
      {
        h: '二、敏感信息保护',
        p: [
          '陪诊相关的病历、诊断证明、医嘱记录等医疗信息单独加密存储，访问留痕，不进入普通日志。',
          '门禁密码与钥匙信息加密存储，仅在服务开始前 2 小时至服务结束后对你与本次服务方可见。',
          '服务记录仅下单用户与本次服务方可见。',
        ],
      },
      {
        h: '三、号码保护',
        p: [
          '联系服务方提供应用内消息与电话两种通道，电话通道使用虚拟号或中间号转接，避免双方号码直接暴露。',
        ],
      },
      {
        h: '四、信息使用范围',
        p: [
          '我们不会将你的个人信息用于服务履约之外的用途，不会向第三方出售或提供，法律法规另有规定的除外。',
        ],
      },
      {
        h: '五、你的权利',
        p: [
          '你可以随时在「设置 - 个人信息」修改昵称与手机号，在「常用地址」增删改地址。',
          '如需注销账号或导出个人信息，可联系客服处理。',
        ],
      },
    ],
  },
  about: {
    title: '关于我们',
    intro: '小区好帮手 —— 把琐事交给我们，家门口的安心生活服务。',
    sections: [
      {
        h: '我们做什么',
        p: [
          '我们只在一个明确的地理范围内，提供三类高频、低决策成本的社区生活服务：代倒垃圾、遛宠、陪诊。',
          '不做全品类撮合，不跨片区服务 —— 因为「家门口」才是信任的来源。',
        ],
      },
      {
        h: '服务范围',
        p: [
          '东方冠郡一期、东方冠郡二期、万科清林径、兴欣花苑、同润欣汇名邸、同润新云都会、招商象屿星耀翠湾、中粮首创禧瑞祥云、丝庐雅苑一期、丝庐雅苑二期、中洲华庭、吉宝汇锦城一期。',
        ],
      },
      {
        h: '服务时段',
        p: ['08:00-10:00 · 12:00-14:00 · 18:00-20:00。遛宠工作日至晚间，周末与节假日全天可约。'],
      },
      {
        h: '联系我们',
        p: ['客服电话：021-0000 0000（工作日 9:00-18:00）'],
      },
    ],
  },
}

const type = ref<DocType>('terms')
const doc = computed(() => DOCS[type.value])

onLoad((query) => {
  const t = (query?.type as string) || 'terms'
  if (t === 'terms' || t === 'privacy' || t === 'about') type.value = t
  uni.setNavigationBarTitle({ title: DOCS[type.value].title })
})
</script>

<template>
  <view class="page">
    <view class="head">
      <view class="title">{{ doc.title }}</view>
      <view class="intro">{{ doc.intro }}</view>
    </view>

    <view v-for="s in doc.sections" :key="s.h" class="card sec">
      <view class="sec-h">{{ s.h }}</view>
      <view v-for="(p, i) in s.p" :key="i" class="para">{{ p }}</view>
    </view>

    <view class="foot">小区好帮手 · v1.0.0</view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 24rpx 24rpx 60rpx;
}

.head {
  padding: 16rpx 8rpx 24rpx;
}

.title {
  font-size: 38rpx;
  font-weight: 800;
  color: var(--text);
}

.intro {
  margin-top: 14rpx;
  font-size: 25rpx;
  color: var(--text-sub);
  line-height: 1.75;
}

.sec {
  padding: 30rpx 32rpx;
  margin-bottom: 20rpx;
}

.sec-h {
  font-size: 29rpx;
  font-weight: 700;
  color: var(--brand-dark);
  margin-bottom: 16rpx;
}

.para {
  font-size: 26rpx;
  color: var(--text-sub);
  line-height: 1.85;
  margin-bottom: 12rpx;
}

.para:last-child {
  margin-bottom: 0;
}

.foot {
  text-align: center;
  font-size: 22rpx;
  color: var(--text-light);
  padding: 24rpx 0 8rpx;
}
</style>
