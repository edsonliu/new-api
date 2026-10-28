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
import { ArrowRight, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import { HomeFooter } from '@/features/jdtoken-home/home-footer'
import { cn } from '@/lib/utils'

import { getApplications } from './catalog'

import styles from './apps.module.css'
import homeStyles from '@/features/jdtoken-home/home.module.css'

export function JdtokenApps() {
  const { t } = useTranslation()
  const applications = getApplications(t)

  return (
    <div className={cn(homeStyles.home, styles.market)}>
      <PublicLayout
        showMainContainer={false}
        headerProps={{ showSystemUpdate: false }}
        siteName={t('JDOTOKEN')}
        logo={
          <img
            src='/jdtoken/mark-square-20261012.png'
            alt=''
            className={homeStyles.brandMark}
          />
        }
      >
        <main id='jdtoken-applications' className={styles.content}>
          <div className={styles.heading}>
            <div>
              <p className={styles.eyebrow}>
                JDOTOKEN · {t('Creative workspace')}
              </p>
              <h1>{t('Apps')}</h1>
            </div>
            <p>{t('Find the right starting point for your next creation.')}</p>
          </div>
          <div className={styles.grid}>
            {applications.map((app) => (
              <Card
                key={app.id}
                role='article'
                aria-labelledby={`app-${app.id}`}
                className={styles.card}
              >
                <CardHeader className={styles.cardHeader}>
                  <div className={styles.cardTitle}>
                    <h2 id={`app-${app.id}`}>{app.title}</h2>
                    <app.icon aria-hidden='true' className={styles.icon} />
                  </div>
                  <Badge variant='secondary' className={styles.status}>
                    <span aria-hidden='true' className={styles.statusDot} />
                    {app.destination ? t('Ready to use') : t('Coming soon')}
                  </Badge>
                </CardHeader>
                <CardContent className={styles.cardContent}>
                  <div className={styles.tags}>
                    {app.tags.map((tag) => (
                      <Badge key={tag} variant='outline'>
                        {tag}
                      </Badge>
                    ))}
                  </div>
                  <p>{app.description}</p>
                </CardContent>
                <CardFooter className={styles.cardFooter}>
                  {app.destination ? (
                    <Button
                      role='link'
                      variant='link'
                      render={<Link to={app.destination} />}
                    >
                      {t('Open playground')}
                      <ArrowRight aria-hidden='true' />
                    </Button>
                  ) : (
                    <Button variant='ghost' disabled>
                      {t('Coming soon')}
                      <Sparkles aria-hidden='true' />
                    </Button>
                  )}
                  {app.id === 'video' && (
                    <Button
                      role='link'
                      variant='link'
                      render={<a href='/docs/#video-workflow' />}
                    >
                      {t('Video API guide')}
                      <ArrowRight aria-hidden='true' />
                    </Button>
                  )}
                  {app.id === 'materials' && (
                    <Button
                      role='link'
                      variant='link'
                      render={<a href='/docs/#asset-mode' />}
                    >
                      {t('Material API guide')}
                      <ArrowRight aria-hidden='true' />
                    </Button>
                  )}
                </CardFooter>
              </Card>
            ))}
          </div>
        </main>
        <HomeFooter />
      </PublicLayout>
    </div>
  )
}
