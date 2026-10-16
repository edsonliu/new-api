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
import { ArrowUpRight, LockKeyhole } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import type { useHomeCatalog } from './catalog'

import styles from './home.module.css'

export function ModelShowcase(props: {
  catalog: ReturnType<typeof useHomeCatalog>
}) {
  const { t } = useTranslation()
  const catalog = props.catalog

  if (catalog.disabled) return null

  let content = (
    <div className={styles.modelGrid}>
      {catalog.featured.map((model) => (
        <article className={styles.modelCard} key={model.model_name}>
          <div className={styles.modelHeading}>
            <span className={styles.modelMark} aria-hidden='true'>
              {model.model_name.split('/').at(-1)?.slice(0, 2).toUpperCase()}
            </span>
            <h3>{model.model_name}</h3>
          </div>
          <p>
            {model.description || t('One gateway for your model workflows.')}
          </p>
          <div className={styles.modelActions}>
            <Badge variant='secondary'>{t('API access')}</Badge>
            <Button
              role='link'
              variant='ghost'
              size='sm'
              render={
                <Link
                  to='/pricing/$modelId'
                  params={{ modelId: model.model_name }}
                />
              }
              aria-label={t('View {{model}} details', {
                model: model.model_name,
              })}
            >
              {t('Details')} <ArrowUpRight aria-hidden='true' />
            </Button>
          </div>
        </article>
      ))}
    </div>
  )

  if (catalog.loading) {
    content = <LoadingState message={t('Loading models...')} />
  } else if (catalog.needsLogin) {
    content = (
      <EmptyState
        icon={LockKeyhole}
        title={t('Sign in to explore available models')}
        action={
          <Button
            role='link'
            render={<Link to='/sign-in' search={{ redirect: '/pricing' }} />}
          >
            {t('Sign in')}
          </Button>
        }
      />
    )
  } else if (catalog.error) {
    content = (
      <ErrorState
        title={t('Models are temporarily unavailable')}
        description={t(
          'You can still explore the platform or try again later.'
        )}
        onRetry={() => {
          void catalog.refetch()
        }}
      />
    )
  } else if (!catalog.featured.length) {
    content = (
      <EmptyState
        title={t('The model catalog is being prepared')}
        description={t('Available models will appear here when published.')}
      />
    )
  }

  return (
    <section
      className={styles.softSection}
      id='models'
      aria-labelledby='jdtoken-models-title'
    >
      <div className={styles.container}>
        <div className={styles.sectionHeading}>
          <p className={styles.eyebrow}>{t('Model Square')}</p>
          <h2 id='jdtoken-models-title'>{t('Model Square')}</h2>
          <p>
            {t('Explore models, compare capabilities, and find your next API.')}
          </p>
        </div>
        <div aria-live='polite'>{content}</div>
        {!catalog.needsLogin &&
          !catalog.error &&
          catalog.featured.length > 0 && (
            <div className={styles.centerAction}>
              <Button
                role='link'
                variant='outline'
                size='lg'
                render={<Link to='/pricing' />}
              >
                {t('Explore all models')} <ArrowUpRight aria-hidden='true' />
              </Button>
            </div>
          )}
      </div>
    </section>
  )
}
