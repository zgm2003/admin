export type ShowcaseItem = {
  image: string
  title: string
  prompt: string
  tags: string[]
}

export const showcaseItems: ShowcaseItem[] = [
  { image: 'https://i.ibb.co/TDFvGWDT/image.png', title: '视觉灵感', prompt: '二次元风格，明亮精致的视觉探索', tags: ['工作', '海报'] },
  { image: 'https://i.ibb.co/zVwJq3YS/image.png', title: '创意参考', prompt: '从已有经验开始新的创作', tags: ['有趣'] },
  { image: 'https://i.ibb.co/PvY3qhhK/image.png', title: '提示词探索', prompt: '记录每一次稳定出图的结果', tags: ['吐槽'] },
  { image: 'https://i.ibb.co/7D04LwN/image.png', title: '风格实验', prompt: '连接图片、文字与图形', tags: ['灵感'] },
]
