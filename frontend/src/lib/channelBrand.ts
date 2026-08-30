import type { Component } from 'vue'
import { Bot } from 'lucide-vue-next'
import WhatsappIcon from '@/components/icons/WhatsappIcon.vue'
import TelegramIcon from '@/components/icons/TelegramIcon.vue'
import InstagramIcon from '@/components/icons/InstagramIcon.vue'
import MessengerIcon from '@/components/icons/MessengerIcon.vue'

// channelBrand is the one source of truth for how a channel renders — icon,
// dot background, text color — shared by ChatList and CustomerPanel so the
// two can never drift into labelling the same channel two different ways
// (or, as with `simulator` before this module existed, one of them not
// labelling it at all: ChatList fell back to WhatsApp's own green for any
// unlisted channel, so a Simulator test conversation rendered visually
// identical to a real WhatsApp one — see KB-12). whatsapp_cloud deliberately
// shares WhatsApp's icon and green: to an operator it IS WhatsApp, only the
// transport underneath differs. simulator gets a distinct violet Bot badge —
// test traffic only, never a real customer channel — matching the Bot icon
// NavRail already uses for the Simulator page itself.
export interface ChannelBrand {
  icon: Component
  dot: string
  text: string
}

export const CHANNEL_BRAND: Record<string, ChannelBrand> = {
  telegram: { icon: TelegramIcon, dot: 'bg-[#229ED9]', text: 'text-[#229ED9]' },
  instagram: { icon: InstagramIcon, dot: 'bg-[#E4405F]', text: 'text-[#E4405F]' },
  messenger: { icon: MessengerIcon, dot: 'bg-[#0084FF]', text: 'text-[#0084FF]' },
  whatsapp: { icon: WhatsappIcon, dot: 'bg-wa', text: 'text-wa' },
  whatsapp_cloud: { icon: WhatsappIcon, dot: 'bg-wa', text: 'text-wa' },
  simulator: { icon: Bot, dot: 'bg-violet-500', text: 'text-violet-600' },
}
const CHANNEL_BRAND_FALLBACK = CHANNEL_BRAND.whatsapp

export function channelIcon(channel: string): Component {
  return (CHANNEL_BRAND[channel] ?? CHANNEL_BRAND_FALLBACK).icon
}
export function channelDot(channel: string): string {
  return (CHANNEL_BRAND[channel] ?? CHANNEL_BRAND_FALLBACK).dot
}
export function channelText(channel: string): string {
  return (CHANNEL_BRAND[channel] ?? CHANNEL_BRAND_FALLBACK).text
}
