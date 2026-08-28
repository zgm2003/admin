import { Drawer } from 'antd'
import { Link } from 'react-router-dom'
import { navigationTools, type NavigationToolSlug } from '@/constant/navigation-tools'

export function MobileNavDrawer({ open, active, onClose }: { open: boolean; active?: NavigationToolSlug; onClose: () => void }) {
  return <Drawer title="Canvas" placement="left" size={280} open={open} onClose={onClose}>
    <nav className="mobile-nav">{navigationTools.map(({ slug, icon: Icon, label }) => <Link key={slug} to={slug === 'canvas' ? '/canvas' : `/${slug}`} onClick={onClose} className={active === slug ? 'active' : ''}><Icon size={18} />{label}</Link>)}</nav>
  </Drawer>
}
