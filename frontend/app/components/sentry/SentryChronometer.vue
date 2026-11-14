<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Power, Radio } from 'lucide-vue-next'
import { useSeed } from '~/composables/useSeed'
import { useI18n } from '~/composables/useI18n'

interface Props {
  size?: 'sm' | 'md' | 'lg'
  interactive?: boolean
  showDigitalTime?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  size: 'md',
  interactive: true,
  showDigitalTime: true
})

const emit = defineEmits<{
  markerClick: [event: 'shutdown' | 'target' | 'standby']
}>()

const { session, isSeedingActive, isSessionStatusKnown, formattedRemainingTime } = useSeed()
const { t } = useI18n()

// Smooth GPU sweep via pure CSS keyframe animation + 1-second cadence for digital time & hour/minute hands
const now = ref(new Date())
const initialSecondOffset = ref(0)
let intervalId: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  const d = new Date()
  initialSecondOffset.value = Number((d.getSeconds() + d.getMilliseconds() / 1000).toFixed(2))
  intervalId = setInterval(() => {
    now.value = new Date()
  }, 1000)
})

onUnmounted(() => {
  if (intervalId) {
    clearInterval(intervalId)
  }
})

// Hour & minute angles update quietly on a 1-second cadence without stressing the DOM
const minutesPrecise = computed(() => now.value.getMinutes() + now.value.getSeconds() / 60)
const hours24Precise = computed(() => now.value.getHours() + minutesPrecise.value / 60)

// Hand angles
// 24-Hour Military Dial: 360 / 24 = 15 degrees per hour
const hourAngle = computed(() => hours24Precise.value * 15)
// Minute Hand: 360 / 60 = 6 degrees per minute
const minuteAngle = computed(() => minutesPrecise.value * 6)
// Fallback initial second angle before ref mounts
const initialSecondAngle = computed(() => (now.value.getSeconds() + now.value.getMilliseconds() / 1000) * 6)

// Digital Formatted Time (HH:MM:SS)
const digitalTimeString = computed(() => {
  const h = String(now.value.getHours()).padStart(2, '0')
  const m = String(now.value.getMinutes()).padStart(2, '0')
  const s = String(now.value.getSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
})

// Event & Mission Scheduling Marker
const targetEndTime = computed<Date | null>(() => {
  if (session.value.state !== 'active' || session.value.remainingSeconds == null) return null
  return new Date(now.value.getTime() + session.value.remainingSeconds * 1000)
})

// Target marker angle on 24-hour military dial: (targetHour + targetMin/60) * 15
const targetMarkerAngle = computed<number | null>(() => {
  if (!targetEndTime.value) return null
  const h = targetEndTime.value.getHours() + targetEndTime.value.getMinutes() / 60 + targetEndTime.value.getSeconds() / 3600
  return h * 15
})

const targetTimeString = computed<string>(() => {
  if (!targetEndTime.value) return ''
  const h = String(targetEndTime.value.getHours()).padStart(2, '0')
  const m = String(targetEndTime.value.getMinutes()).padStart(2, '0')
  const s = String(targetEndTime.value.getSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
})

const plannedActionLabel = computed(() => {
  if (!isSessionStatusKnown.value) return t('common.unknown')
  if (!isSeedingActive.value) return t('dashboard.standby')
  const postSeed = session.value.plan?.postSeedAction
  if (postSeed === 'close_squad') return t('dashboard.actionCloseSquad')
  if (postSeed === 'shutdown') return t('dashboard.actionShutdown')
  if (postSeed === 'sleep') return t('dashboard.actionSleep')
  return t('dashboard.targetReachedLabel')
})

const dialDimension = computed(() => {
  switch (props.size) {
    case 'sm': return { px: 210, viewBox: 320 }
    case 'lg': return { px: 340, viewBox: 320 }
    case 'md':
    default: return { px: 270, viewBox: 320 }
  }
})

// 60 Minute/Second ticks
const minuteTicks = computed(() => {
  const result = []
  for (let i = 0; i < 60; i++) {
    result.push({
      index: i,
      angle: i * 6,
      isMajor: i % 5 === 0,
      isCardinal: i % 15 === 0
    })
  }
  return result
})

// 24-Hour Markers (Every 2 hours marked: 00, 02, 04, 06, 08, 10, 12, 14, 16, 18, 20, 22)
const hourMarks24 = computed(() => {
  const labels = ['00', '02', '04', '06', '08', '10', '12', '14', '16', '18', '20', '22']
  return labels.map((text, idx) => {
    const hourVal = idx * 2
    const angle = hourVal * 15 // 15 deg per hour
    const rad = ((angle - 90) * Math.PI) / 180
    const r = 104 // Dedicated military hour radius
    return {
      text,
      hour: hourVal,
      angle,
      x: 160 + r * Math.cos(rad),
      y: 160 + r * Math.sin(rad) + 4 // font baseline offset
    }
  })
})

// Dedicated 5-minute indicators (00, 05, 10, 15, ..., 55)
const minuteLabels = computed(() => {
  const list = ['00', '05', '10', '15', '20', '25', '30', '35', '40', '45', '50', '55']
  return list.map((text, idx) => {
    const minuteVal = idx * 5
    const angle = minuteVal * 6
    const rad = ((angle - 90) * Math.PI) / 180
    const r = 138 // Outer minute track
    return {
      text,
      x: 160 + r * Math.cos(rad),
      y: 160 + r * Math.sin(rad) + 3.5
    }
  })
})
</script>

<template>
  <div class="relative flex flex-col items-center justify-center select-none group">
    <!-- Outer Ambient Tactical Glow -->
    <div
      class="absolute rounded-full pointer-events-none transition-all duration-700"
      :class="[
        isSeedingActive
          ? 'w-64 h-64 bg-[radial-gradient(circle,rgba(255,153,0,0.15)_0%,transparent_70%)]'
          : 'w-60 h-60 bg-[radial-gradient(circle,rgba(235,225,200,0.1)_0%,transparent_70%)]'
      ]"
    />

    <!-- Main Circular Dial -->
    <div
      class="relative rounded-full overflow-hidden border border-accent/40 bg-[#05070d] shadow-xl shadow-black/80 flex items-center justify-center transition-transform duration-300"
      :style="{
        width: `${dialDimension.px}px`,
        height: `${dialDimension.px}px`
      }"
    >
      <!-- Background Sentry Emblem -->
      <img
        src="/img/citadel_sentry_gold.jpg"
        alt="Citadel Sentry Dial"
        class="absolute inset-0 w-full h-full object-cover rounded-full opacity-30 pointer-events-none"
      />

      <!-- Radial Vignette Mask -->
      <div
        class="absolute inset-0 rounded-full pointer-events-none"
        style="background: radial-gradient(circle, transparent 35%, rgba(5,7,13,0.7) 70%, rgba(4,5,9,0.96) 100%);"
      />

      <!-- Radar Scanning Sweep Cone (Hardware accelerated GPU rotation) -->
      <div
        class="absolute inset-0 rounded-full pointer-events-none opacity-20 sentry-radar-sweep"
        :style="{
          animationDelay: `-${initialSecondOffset}s`,
          background: 'conic-gradient(from 0deg, transparent 0deg, transparent 290deg, rgba(235,225,200,0.3) 345deg, rgba(255,153,0,0.5) 360deg)'
        }"
      />

      <!-- SVG Tactical 24H Military Clock Face & Hands -->
      <svg
        class="absolute inset-0 w-full h-full pointer-events-none"
        viewBox="0 0 320 320"
      >
        <!-- Outer Minute Ring -->
        <circle
          cx="160"
          cy="160"
          r="150"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          class="text-accent/35"
        />

        <!-- Secondary Track dividing Minutes and 24-Hours -->
        <circle
          cx="160"
          cy="160"
          r="124"
          fill="none"
          stroke="currentColor"
          stroke-width="1"
          stroke-dasharray="1 3"
          class="text-accent/25"
        />

        <!-- Inner Military Track -->
        <circle
          cx="160"
          cy="160"
          r="86"
          fill="none"
          stroke="currentColor"
          stroke-width="0.75"
          class="text-accent/20"
        />

        <!-- 60 Minute/Second Precision Ticks on Outer Rim -->
        <g v-for="t in minuteTicks" :key="t.index" :transform="`rotate(${t.angle} 160 160)`">
          <line
            v-if="t.isCardinal"
            x1="160"
            y1="10"
            x2="160"
            y2="20"
            stroke="currentColor"
            stroke-width="2"
            class="text-accent"
          />
          <line
            v-else-if="t.isMajor"
            x1="160"
            y1="10"
            x2="160"
            y2="17"
            stroke="currentColor"
            stroke-width="1.2"
            class="text-accent/60"
          />
          <line
            v-else
            x1="160"
            y1="10"
            x2="160"
            y2="14"
            stroke="currentColor"
            stroke-width="0.7"
            class="text-accent/25"
          />
        </g>

        <!-- Dedicated Minute Labels (00, 05, 10, ... 55) -->
        <text
          v-for="ml in minuteLabels"
          :key="ml.text"
          :x="ml.x"
          :y="ml.y"
          text-anchor="middle"
          class="fill-accent/70 text-[8px] font-mono font-medium tracking-tight"
        >
          {{ ml.text }}
        </text>

        <!-- 24-Hour Military Radial Dial Numbers (00, 02, 04, ..., 22) -->
        <text
          v-for="hm in hourMarks24"
          :key="hm.text"
          :x="hm.x"
          :y="hm.y"
          text-anchor="middle"
          :class="[
            'font-mono font-bold tracking-wider',
            hm.hour === 0 ? 'fill-accent text-[11px]' :
            (hm.hour % 6 === 0 ? 'fill-accent/90 text-[10px]' : 'fill-accent/60 text-[9px]')
          ]"
        >
          {{ hm.text }}
        </text>

        <!-- 24-Hour Intermediate Ticks (Odd hours: 01, 03, 05, etc.) -->
        <g v-for="h in 24" :key="`hour-tick-${h}`" :transform="`rotate(${h * 15} 160 160)`">
          <circle
            cx="160"
            cy="124"
            r="1"
            fill="currentColor"
            class="text-accent/40"
          />
        </g>

        <!-- TARGET / MISSION END SCHEDULE MARKER -->
        <g v-if="targetMarkerAngle !== null" :transform="`rotate(${targetMarkerAngle} 160 160)`">
          <!-- Marker Ray Line -->
          <line
            x1="160"
            y1="10"
            x2="160"
            y2="34"
            stroke="currentColor"
            stroke-width="2.5"
            class="text-squad-orange"
          />
          <!-- Outer Pulsing Sentry Chevron Beacon -->
          <polygon
            points="160,8 165,16 155,16"
            fill="currentColor"
            class="text-squad-orange animate-pulse"
          />
          <!-- Event Marker Dot -->
          <circle
            cx="160"
            cy="24"
            r="3"
            fill="currentColor"
            class="text-squad-orange animate-ping"
          />
        </g>

        <!-- CLOCK HANDS (Buttery Smooth 60 FPS continuous Glide) -->

        <!-- 24-Hour Hand (Chamfered Tactical Military Needle) -->
        <g :transform="`rotate(${hourAngle} 160 160)`">
          <!-- Hour shadow -->
          <line x1="160" y1="160" x2="160" y2="88" stroke="#000" stroke-width="5" stroke-linecap="round" opacity="0.6" />
          <!-- Hour body -->
          <line x1="160" y1="160" x2="160" y2="90" stroke="#EBE1C8" stroke-width="3.5" stroke-linecap="round" />
          <!-- Luminous insert -->
          <line x1="160" y1="145" x2="160" y2="98" stroke="#FFF" stroke-width="1.2" stroke-linecap="round" opacity="0.9" />
          <!-- Counterweight -->
          <line x1="160" y1="160" x2="160" y2="175" stroke="#EBE1C8" stroke-width="3" stroke-linecap="round" />
        </g>

        <!-- Minute Hand (Golden Brass Slender Needle) -->
        <g :transform="`rotate(${minuteAngle} 160 160)`">
          <!-- Minute shadow -->
          <line x1="160" y1="160" x2="160" y2="52" stroke="#000" stroke-width="4" stroke-linecap="round" opacity="0.6" />
          <!-- Minute body -->
          <line x1="160" y1="160" x2="160" y2="54" stroke="#D8C49D" stroke-width="2.5" stroke-linecap="round" />
          <!-- Luminous line -->
          <line x1="160" y1="145" x2="160" y2="62" stroke="#FFF" stroke-width="1" stroke-linecap="round" opacity="0.9" />
          <!-- Counterweight -->
          <line x1="160" y1="160" x2="160" y2="180" stroke="#D8C49D" stroke-width="2" stroke-linecap="round" />
        </g>

        <!-- Second Hand (Squad Orange Laser Needle with Glowing Pointer - Hardware GPU animated) -->
        <g
          class="sentry-second-sweep"
          :style="{
            animationDelay: `-${initialSecondOffset}s`
          }"
        >
          <!-- Counterweight tail -->
          <line x1="160" y1="160" x2="160" y2="192" stroke="#FF9900" stroke-width="1.2" />
          <circle cx="160" cy="182" r="3" fill="#FF9900" />
          <!-- Sweeping shaft -->
          <line x1="160" y1="160" x2="160" y2="28" stroke="#FF9900" stroke-width="1.2" />
          <!-- Glowing tip indicator -->
          <circle cx="160" cy="30" r="2.5" fill="#FFF" stroke="#FF9900" stroke-width="1.2" />
        </g>

        <!-- Center Hub: Tactical Pivot Jewel -->
        <circle cx="160" cy="160" r="8" fill="#1A1D24" stroke="#D8C49D" stroke-width="1.8" />
        <circle cx="160" cy="160" r="3.5" fill="#FF9900" class="animate-pulse" />
      </svg>
    </div>

    <!-- Telemetry & Digital Time Sub-Banner (Placed neatly outside dial to avoid clipping) -->
    <div
      v-if="showDigitalTime"
      class="mt-3.5 text-center space-y-1.5 font-mono"
    >
      <!-- Digital Tactical Badge -->
      <div
        v-if="interactive"
        class="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-surface border border-accent/30 text-xs font-bold tracking-wider text-accent shadow-lg cursor-pointer hover:border-accent transition-colors"
        @click="emit('markerClick', targetEndTime ? 'shutdown' : 'standby')"
      >
        <span class="w-2 h-2 rounded-full" :class="isSeedingActive ? 'bg-squad-orange animate-ping' : 'bg-emerald-400'" />
        <span>{{ digitalTimeString }}</span>
        <span class="text-[10px] text-accent/60 uppercase">UTC+{{ -new Date().getTimezoneOffset() / 60 }}</span>
      </div>

      <!-- Active Event / Target Indicator -->
      <div v-if="targetEndTime" class="block">
        <div class="inline-flex items-center space-x-2 px-3 py-1 rounded-lg bg-squad-orange/10 border border-squad-orange/30 text-squad-orange text-xs shadow-md">
          <Power class="w-3.5 h-3.5" />
          <span class="font-bold tracking-wide uppercase">{{ plannedActionLabel }}:</span>
          <span class="font-bold text-white">{{ targetTimeString }}</span>
          <span v-if="formattedRemainingTime" class="text-primary/70 text-[11px]">({{ formattedRemainingTime }})</span>
        </div>
      </div>

      <!-- Standby Watch Indicator -->
      <div v-else class="block">
        <div class="inline-flex items-center space-x-2 px-3 py-1 rounded-lg bg-surface border border-accent-secondary/20 text-primary/70 text-xs shadow-md">
          <Radio class="w-3.5 h-3.5 text-accent animate-pulse" />
          <span class="tracking-wider uppercase">{{ t('dashboard.sentryWatch') }}:</span>
          <span class="font-bold text-accent">{{ t('dashboard.active') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes sentry-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.sentry-radar-sweep {
  transform-origin: center;
  animation: sentry-spin 60s linear infinite;
  will-change: transform;
}

.sentry-second-sweep {
  transform-box: view-box;
  transform-origin: 160px 160px;
  animation: sentry-spin 60s linear infinite;
  will-change: transform;
}
</style>
