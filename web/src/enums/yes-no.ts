export const YesNo = {
  No: 0,
  Yes: 1,
} as const

export type YesNo = (typeof YesNo)[keyof typeof YesNo]

export function isYesNo(value: unknown): value is YesNo {
  return value === YesNo.No || value === YesNo.Yes
}
