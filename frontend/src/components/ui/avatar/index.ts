import { type VariantProps, cva } from 'class-variance-authority'

export { default as Avatar } from './Avatar.vue'
export { default as AvatarFallback } from './AvatarFallback.vue'
export { default as AvatarImage } from './AvatarImage.vue'

export const avatarVariant = cva(
  'inline-flex shrink-0 select-none items-center justify-center overflow-hidden bg-secondary font-semibold text-foreground',
  {
    variants: {
      size: {
        sm: 'h-9 w-9 text-xs',
        base: 'h-10 w-10 text-sm',
        lg: 'h-11 w-11 text-sm',
      },
      shape: {
        circle: 'rounded-full',
        square: 'rounded-md',
      },
    },
    defaultVariants: {
      size: 'base',
      shape: 'circle',
    },
  },
)

export type AvatarVariants = VariantProps<typeof avatarVariant>
