/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight, ArrowUpRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

import { useHomeCatalog } from './catalog'
import { HomeFooter } from './home-footer'
import { ModelShowcase } from './model-showcase'
import { PlatformSections } from './platform-sections'
import { Quickstart } from './quickstart'

import styles from './home.module.css'

export default function JdtokenHome() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { status } = useStatus()
  const catalog = useHomeCatalog()
  let startPath: '/dashboard' | '/sign-in' | '/sign-up' = '/sign-in'
  let startLabel = t('Sign in')
  if (user) {
    startPath = '/dashboard'
    startLabel = t('Go to Dashboard')
  } else if (status?.register_enabled) {
    startPath = '/sign-up'
    startLabel = t('Get started with JDOTOKEN')
  }

  return (
    <div className={styles.home}>
      <PublicLayout
        showMainContainer={false}
        headerProps={{ showSystemUpdate: false }}
        siteName={t('JDOTOKEN')}
        logo={
          <img
            src='/jdtoken/mark-square-20261012.png'
            alt=''
            className={styles.brandMark}
          />
        }
      >
        <main id='jdtoken-content'>
          <section className={styles.hero} aria-labelledby='jdtoken-hero-title'>
            <img
              src='/jdtoken/hero.webp'
              alt=''
              className={styles.heroImage}
              width={2400}
              height={690}
              fetchPriority='high'
            />
            <div className={styles.heroContent}>
              <p className={styles.eyebrow}>
                JDOTOKEN · {t('One API gateway')}
              </p>
              <h1 id='jdtoken-hero-title'>{t('Model access, made simple.')}</h1>
              <p className={styles.heroDescription}>
                {t(
                  'Bring leading AI models together through one API. Build faster, manage usage, and keep your applications connected.'
                )}
              </p>
              <div className={styles.heroActions}>
                <Button role='link' size='lg' render={<Link to={startPath} />}>
                  {startLabel}
                  <ArrowRight aria-hidden='true' />
                </Button>
                {!catalog.disabled && (
                  <Button
                    role='link'
                    variant='outline'
                    size='lg'
                    render={<Link to='/pricing' />}
                  >
                    {t('Model Square')}
                    <ArrowUpRight aria-hidden='true' />
                  </Button>
                )}
              </div>
            </div>
          </section>
          <PlatformSections />
          <ModelShowcase catalog={catalog} />
          <Quickstart model={catalog.chatModel} />
          <section
            className={styles.section}
            aria-labelledby='jdtoken-start-title'
          >
            <div className={`${styles.container} ${styles.cta}`}>
              <p className={styles.eyebrow}>{t('Get Started')}</p>
              <h2 id='jdtoken-start-title'>{t('Ready for your next idea?')}</h2>
              <p>{t('Your models, keys, and usage. All in one workspace.')}</p>
              <div className={styles.heroActions}>
                <Button role='link' size='lg' render={<Link to={startPath} />}>
                  {startLabel}
                  <ArrowRight aria-hidden='true' />
                </Button>
                <Button
                  role='link'
                  variant='outline'
                  size='lg'
                  render={<Link to='/playground' />}
                >
                  {t('Try the playground')}
                </Button>
              </div>
              <Link to='/wallet' className={styles.textLink}>
                {t('Explore plans and manage your balance')}
              </Link>
            </div>
          </section>
        </main>
        <HomeFooter />
      </PublicLayout>
    </div>
  )
}
