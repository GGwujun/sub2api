<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <!-- Empty State -->
      <div v-else-if="subscriptions.length === 0" class="card p-12 text-center">
        <div
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <Icon name="creditCard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <!-- Subscriptions Grid -->
      <div v-else class="grid gap-6 lg:grid-cols-2">
        <div
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="card overflow-hidden"
        >
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
          >
            <div class="flex items-center gap-3">
              <div
                class="flex h-10 w-10 items-center justify-center rounded-xl bg-purple-100 dark:bg-purple-900/30"
              >
                <Icon name="creditCard" size="md" class="text-purple-600 dark:text-purple-400" />
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">
                  {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                </h3>
                <p class="text-xs text-gray-500 dark:text-dark-400">
                  {{ subscription.group?.description || '' }}
                </p>
              </div>
            </div>
            <span
              :class="[
                'badge',
                subscription.status === 'active'
                  ? 'badge-success'
                  : subscription.status === 'expired'
                    ? 'badge-warning'
                    : 'badge-danger'
              ]"
            >
              {{ t(`userSubscriptions.status.${subscription.status}`) }}
            </span>
          </div>

          <!-- Usage Progress -->
          <div class="space-y-4 p-4">
            <!-- Expiration Info -->
            <div v-if="subscription.expires_at" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span :class="getExpirationClass(subscription.expires_at)">
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-gray-700 dark:text-gray-300">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <!-- Token Quota Limits -->
            <div v-if="isTokenQuotaSubscription(subscription)">
              <div v-if="hasTokenQuotaLimits(subscription)" class="space-y-4">
                <div
                  v-if="subscription.group?.token_quota && subscription.group.token_quota > 0"
                  class="space-y-2"
                >
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.tokenTotal') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      {{
                        formatTokenUsage(
                          getTokenUsageTotal(subscription),
                          subscription.group.token_quota
                        )
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          getTokenUsageTotal(subscription),
                          subscription.group.token_quota
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          getTokenUsageTotal(subscription),
                          subscription.group.token_quota
                        )
                      }"
                    ></div>
                  </div>
                </div>
                <div
                  v-if="subscription.group?.token_quota_daily && subscription.group.token_quota_daily > 0"
                  class="space-y-2"
                >
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.tokenDaily') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      {{
                        formatTokenUsage(
                          getTokenUsageDaily(subscription),
                          subscription.group.token_quota_daily
                        )
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          getTokenUsageDaily(subscription),
                          subscription.group.token_quota_daily
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          getTokenUsageDaily(subscription),
                          subscription.group.token_quota_daily
                        )
                      }"
                    ></div>
                  </div>
                </div>
                <div
                  v-if="subscription.group?.token_quota_weekly && subscription.group.token_quota_weekly > 0"
                  class="space-y-2"
                >
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.tokenWeekly') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      {{
                        formatTokenUsage(
                          getTokenUsageWeekly(subscription),
                          subscription.group.token_quota_weekly
                        )
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          getTokenUsageWeekly(subscription),
                          subscription.group.token_quota_weekly
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          getTokenUsageWeekly(subscription),
                          subscription.group.token_quota_weekly
                        )
                      }"
                    ></div>
                  </div>
                </div>
                <div
                  v-if="subscription.group?.token_quota_monthly && subscription.group.token_quota_monthly > 0"
                  class="space-y-2"
                >
                  <div class="flex items-center justify-between">
                    <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                      {{ t('userSubscriptions.tokenMonthly') }}
                    </span>
                    <span class="text-sm text-gray-500 dark:text-dark-400">
                      {{
                        formatTokenUsage(
                          getTokenUsageMonthly(subscription),
                          subscription.group.token_quota_monthly
                        )
                      }}
                    </span>
                  </div>
                  <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                    <div
                      class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                      :class="
                        getProgressBarClass(
                          getTokenUsageMonthly(subscription),
                          subscription.group.token_quota_monthly
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          getTokenUsageMonthly(subscription),
                          subscription.group.token_quota_monthly
                        )
                      }"
                    ></div>
                  </div>
                </div>
              </div>
              <div
                v-else
                class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
              >
                <div class="flex items-center gap-3">
                  <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                  <div>
                    <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                      {{ t('userSubscriptions.unlimited') }}
                    </p>
                    <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                      {{ t('userSubscriptions.unlimitedDesc') }}
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <!-- Daily Usage -->
            <div v-if="!isTokenQuotaSubscription(subscription) && subscription.group?.daily_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.daily_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.daily_usage_usd,
                      subscription.group.daily_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.daily_usage_usd,
                      subscription.group.daily_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.daily_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.daily_window_start, 24)
                  })
                }}
              </p>
            </div>

            <!-- Weekly Usage -->
            <div v-if="!isTokenQuotaSubscription(subscription) && subscription.group?.weekly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.weekly_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.weekly_usage_usd,
                      subscription.group.weekly_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.weekly_usage_usd,
                      subscription.group.weekly_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.weekly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.weekly_window_start, 168)
                  })
                }}
              </p>
            </div>

            <!-- Monthly Usage -->
            <div v-if="!isTokenQuotaSubscription(subscription) && subscription.group?.monthly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <span class="text-sm text-gray-500 dark:text-dark-400">
                  ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.monthly_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.monthly_usage_usd,
                      subscription.group.monthly_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.monthly_usage_usd,
                      subscription.group.monthly_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.monthly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.monthly_window_start, 720)
                  })
                }}
              </p>
            </div>

            <!-- No limits configured - Unlimited badge -->
            <div
              v-if="
                !isTokenQuotaSubscription(subscription) &&
                !subscription.group?.daily_limit_usd &&
                !subscription.group?.weekly_limit_usd &&
                !subscription.group?.monthly_limit_usd
              "
              class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                <div>
                  <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateOnly, formatTokensK } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function isTokenQuotaSubscription(subscription: UserSubscription): boolean {
  return subscription.group?.subscription_type === 'token_quota'
}

function hasTokenQuotaLimits(subscription: UserSubscription): boolean {
  const group = subscription.group
  return !!(group?.token_quota && group.token_quota > 0) ||
    !!(group?.token_quota_daily && group.token_quota_daily > 0) ||
    !!(group?.token_quota_weekly && group.token_quota_weekly > 0) ||
    !!(group?.token_quota_monthly && group.token_quota_monthly > 0)
}

function getTokenUsageTotal(subscription: UserSubscription): number {
  // Token 配额订阅：使用订阅层的 token 使用量
  if (isTokenQuotaSubscription(subscription)) {
    return subscription.token_usage_total || 0
  }
  return 0
}

function getTokenUsageDaily(subscription: UserSubscription): number {
  // Token 配额订阅：使用订阅层的 token 使用量
  if (isTokenQuotaSubscription(subscription)) {
    return subscription.token_usage_daily || 0
  }
  return subscription.daily_usage_usd || 0
}

function getTokenUsageWeekly(subscription: UserSubscription): number {
  // Token 配额订阅：使用订阅层的 token 使用量
  if (isTokenQuotaSubscription(subscription)) {
    return subscription.token_usage_weekly || 0
  }
  return subscription.weekly_usage_usd || 0
}

function getTokenUsageMonthly(subscription: UserSubscription): number {
  // Token 配额订阅：使用订阅层的 token 使用量
  if (isTokenQuotaSubscription(subscription)) {
    return subscription.token_usage_monthly || 0
  }
  return subscription.monthly_usage_usd || 0
}

function formatTokenUsage(used: number | undefined, limit: number | null | undefined): string {
  const usedValue = formatTokensK(Math.max(0, Math.round(used || 0)))
  const limitValue = limit && limit > 0 ? formatTokensK(limit) : '∞'
  return `${usedValue} / ${limitValue}`
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateOnly(expires)

  if (days === 0) {
    return `${dateStr} (Today)`
  }
  if (days === 1) {
    return `${dateStr} (Tomorrow)`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const now = new Date()
  const diff = end.getTime() - now.getTime()

  if (diff <= 0) return t('userSubscriptions.windowNotActive')

  const hours = Math.floor(diff / (1000 * 60 * 60))
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60))

  if (hours > 24) {
    const days = Math.floor(hours / 24)
    const remainingHours = hours % 24
    return `${days}d ${remainingHours}h`
  }

  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }

  return `${minutes}m`
}

onMounted(() => {
  loadSubscriptions()
})
</script>
