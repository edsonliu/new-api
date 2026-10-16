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
import {
  ArrowLeftRight,
  Check,
  Gauge,
  Layers3,
  ShieldCheck,
  Workflow,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import styles from './home.module.css'

export function PlatformSections() {
  const { t } = useTranslation()
  const capabilities = [
    { value: 'API', label: t('One gateway, multiple models') },
    { value: 'Token', label: t('Track usage as you build') },
    { value: 'Key', label: t('Manage access in one place') },
    { value: 'SDK', label: t('Connect with familiar tools') },
  ]
  const overview = [
    {
      icon: Layers3,
      title: t('A connected model catalog'),
      description: t(
        'Discover model capabilities and choose the right fit for each task.'
      ),
    },
    {
      icon: ArrowLeftRight,
      title: t('One API gateway'),
      description: t(
        'Keep model access and API credentials together in one workspace.'
      ),
    },
    {
      icon: Workflow,
      title: t('From discovery to production'),
      description: t(
        'Move from choosing a model to creating a key and reviewing usage.'
      ),
    },
  ]
  const benefits = [
    {
      icon: Gauge,
      title: t('Clear pricing and usage'),
      description: t(
        'Compare current model pricing and review your consumption in the console.'
      ),
    },
    {
      icon: Zap,
      title: t('A simpler integration'),
      description: t(
        'Use familiar SDKs and a consistent gateway address for your applications.'
      ),
    },
    {
      icon: ShieldCheck,
      title: t('Access you can manage'),
      description: t(
        'Create and manage API keys, then follow their usage from your workspace.'
      ),
    },
  ]

  return (
    <>
      <section className={styles.section} aria-labelledby='jdtoken-value-title'>
        <div className={styles.container}>
          <div className={styles.sectionHeading}>
            <h2 id='jdtoken-value-title'>{t('Turn tokens into real value')}</h2>
            <p>
              {t(
                'One entry point for model access, usage, and your next idea.'
              )}
            </p>
          </div>
          <dl className={styles.capabilities}>
            {capabilities.map((item) => (
              <div key={item.value}>
                <dt>{item.label}</dt>
                <dd>{item.value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </section>
      <section
        className={styles.softSection}
        id='about'
        aria-labelledby='jdtoken-about-title'
      >
        <div className={`${styles.container} ${styles.aboutGrid}`}>
          <div>
            <p className={styles.eyebrow}>{t('About')}</p>
            <h2 id='jdtoken-about-title'>
              {t('JDOTOKEN, your model API workspace')}
            </h2>
            <p className={styles.prose}>
              {t(
                'JDOTOKEN brings model discovery, API access, and usage management into one place. Built for developers and teams, it helps you move from an idea to your first model call.'
              )}
            </p>
          </div>
          <div className={styles.overviewGrid}>
            {overview.map((item) => (
              <article key={item.title} className={styles.overviewCard}>
                <item.icon aria-hidden='true' />
                <div>
                  <h3>{item.title}</h3>
                  <p>{item.description}</p>
                </div>
              </article>
            ))}
          </div>
        </div>
      </section>
      <section
        className={styles.section}
        aria-labelledby='jdtoken-benefits-title'
      >
        <div className={styles.container}>
          <div className={styles.sectionHeading}>
            <h2 id='jdtoken-benefits-title'>
              {t('Make AI part of your everyday work')}
            </h2>
            <p>{t('Less integration work. More room for your ideas.')}</p>
          </div>
          <div className={styles.benefitGrid}>
            {benefits.map((item) => (
              <article className={styles.benefitCard} key={item.title}>
                <item.icon aria-hidden='true' className={styles.featureIcon} />
                <h3>{item.title}</h3>
                <p>{item.description}</p>
                <Check aria-hidden='true' className={styles.checkIcon} />
              </article>
            ))}
          </div>
        </div>
      </section>
    </>
  )
}
