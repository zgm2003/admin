import { ArrowRight } from 'lucide-react'
import { type ReactNode } from 'react'
import { Button, Tag } from 'antd'
import { useNavigate } from 'react-router-dom'

function Highlighter({ action, color, children }: { action: 'highlight' | 'underline'; color: string; children?: ReactNode }) {
  return <span className="relative inline-block px-1">
    <span className="absolute inset-x-0 bottom-0 top-1 rounded-sm opacity-45" style={action === 'highlight' ? { backgroundColor: color } : undefined} />
    {action === 'underline' ? <span className="absolute inset-x-0 bottom-0 h-1 rounded-full opacity-80" style={{ backgroundColor: color }} /> : null}
    <span className="relative font-medium text-stone-800 dark:text-stone-200">{children}</span>
  </span>
}

const showcase = [
  { image: 'https://i.ibb.co/TDFvGWDT/image.png', title: '视觉灵感', prompt: '二次元风格，明亮精致的视觉探索', tags: ['工作', '海报'] },
  { image: 'https://i.ibb.co/zVwJq3YS/image.png', title: '创意参考', prompt: '从已有经验开始新的创作', tags: ['有趣'] },
  { image: 'https://i.ibb.co/PvY3qhhK/image.png', title: '提示词探索', prompt: '记录每一次稳定出图的结果', tags: ['吐槽'] },
  { image: 'https://i.ibb.co/7D04LwN/image.png', title: '风格实验', prompt: '连接图片、文字与图形', tags: ['灵感'] },
]

export default function HomePage() {
  const navigate = useNavigate()
  return <main className="relative h-full overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] text-stone-950 dark:bg-[radial-gradient(rgba(245,245,244,.18)_1px,transparent_1px)] dark:text-stone-100">
    <section className="relative mx-auto min-h-[calc(100vh-4rem)] max-w-7xl overflow-hidden px-6">
      <div className="pointer-events-none absolute left-[15%] top-24 size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />
      <div className="pointer-events-none absolute right-[23%] top-[48%] size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />
      <div className="relative flex min-h-[620px] flex-col items-center justify-center pt-10 text-center">
        <h1 className="ai-title-aurora max-w-5xl text-balance text-5xl font-semibold tracking-normal sm:text-7xl lg:text-8xl">无限画布</h1>
        <p className="mt-8 max-w-3xl text-balance text-lg leading-8 text-stone-500 dark:text-stone-400">在 <Highlighter action="underline" color="#FF9800">无限画布</Highlighter> 中生成、连接和重组 <Highlighter action="highlight" color="#87CEFA">图片、文字与图形</Highlighter>，让创作从单次生成变成连续推演。</p>
        <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
          <Button type="primary" size="large" onClick={() => navigate('/canvas')} icon={<ArrowRight className="size-4" />} iconPlacement="end">开始使用</Button>
          <Button size="large" onClick={() => navigate('/canvas')}>打开画布</Button>
        </div>
      </div>
      <section className="relative mx-auto mb-20 max-w-6xl border-t border-stone-200 pt-12 dark:border-stone-800">
        <div className="mb-8 grid gap-4 md:grid-cols-[1fr_auto_1fr] md:items-start"><div /><div className="max-w-2xl text-center"><h2 className="text-3xl font-semibold text-stone-950 dark:text-stone-100">沉淀每一次好结果</h2><p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">收藏稳定出图的提示词、参考风格和结果图片，让下一次创作从已有经验开始。</p></div><Button type="link" onClick={() => navigate('/prompts')} className="justify-self-center md:justify-self-end" icon={<ArrowRight className="size-4" />} iconPlacement="end">查看提示词库</Button></div>
        <div className="grid auto-rows-[210px] gap-4 md:grid-cols-4">{showcase.map((item, index) => <button key={item.image} type="button" onClick={() => navigate('/prompts')} className={`group relative cursor-pointer overflow-hidden border border-stone-200 bg-stone-100 text-left dark:border-stone-800 dark:bg-stone-900 ${index === 0 ? 'md:col-span-2 md:row-span-2' : ''} ${index === 3 ? 'md:col-span-2' : ''}`}><img src={item.image} alt={item.title} className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]" /><div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/35 to-transparent p-4 text-white"><div className="mb-2 flex flex-wrap gap-1.5">{item.tags.map((tag) => <Tag key={tag} className="m-0 bg-white/15 text-[11px] text-white backdrop-blur">{tag}</Tag>)}</div><h3 className="text-sm font-medium">{item.title}</h3><p className="mt-1 line-clamp-2 text-xs leading-5 text-white/75">{item.prompt}</p></div></button>)}</div>
      </section>
    </section>
  </main>
}
