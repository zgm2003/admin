import { FileText, ImagePlus, Images, Maximize2, Settings2, Video } from 'lucide-react'

export const navigationTools = [
  { slug: 'canvas', icon: Maximize2, label: '我的画布' },
  { slug: 'image', icon: ImagePlus, label: '生图工作台' },
  { slug: 'video', icon: Video, label: '视频创作台' },
  { slug: 'prompts', icon: FileText, label: '提示词库' },
  { slug: 'assets', icon: Images, label: '我的资产' },
  { slug: 'config', icon: Settings2, label: '配置' },
] as const

export type NavigationToolSlug = (typeof navigationTools)[number]['slug']
