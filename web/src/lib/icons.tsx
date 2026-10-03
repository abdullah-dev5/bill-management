import {
  Baby, BookOpen, Building2, Car, Dog, Droplet, Droplets, Dumbbell, Ellipsis, Flame, Fuel, Gift,
  GraduationCap, Hammer, HeartPulse, House, Landmark, PaintRoller, PlugZap, Receipt, Refrigerator,
  Repeat, Shield, ShoppingCart, Smartphone, Sparkles, Store, Tv, Umbrella, User, Wifi, Wrench, Zap,
  type LucideIcon, type LucideProps,
} from 'lucide-react'

/** Icons a bill type or profile may use, keyed by their Lucide name (stored in the database). */
export const ICONS: Record<string, LucideIcon> = {
  zap: Zap,
  flame: Flame,
  droplet: Droplet,
  wifi: Wifi,
  smartphone: Smartphone,
  tv: Tv,
  house: House,
  wrench: Wrench,
  hammer: Hammer,
  sparkles: Sparkles,
  shield: Shield,
  refrigerator: Refrigerator,
  umbrella: Umbrella,
  'graduation-cap': GraduationCap,
  repeat: Repeat,
  landmark: Landmark,
  receipt: Receipt,
  ellipsis: Ellipsis,
  'plug-zap': PlugZap,
  droplets: Droplets,
  'paint-roller': PaintRoller,
  car: Car,
  user: User,
  'heart-pulse': HeartPulse,
  baby: Baby,
  dog: Dog,
  store: Store,
  'building-2': Building2,
  fuel: Fuel,
  'shopping-cart': ShoppingCart,
  'book-open': BookOpen,
  dumbbell: Dumbbell,
  gift: Gift,
}

export const ICON_NAMES = Object.keys(ICONS)

export function NamedIcon({ name, ...props }: { name: string } & LucideProps) {
  const Icon = ICONS[name] ?? Receipt
  return <Icon strokeWidth={1.75} aria-hidden {...props} />
}
