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
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'

export function HomeFooter() {
  const { t } = useTranslation()
  const { status } = useStatus()

  return (
    <footer className='border-border relative border-t'>
      <div className='mx-auto flex max-w-6xl flex-col gap-6 px-6 py-10 sm:flex-row sm:items-center sm:justify-between'>
        <Link
          to='/'
          className='flex shrink-0 items-center gap-2.5 text-sm font-semibold'
          aria-label={t('JDOTOKEN')}
        >
          <img
            src='/jdtoken/logo-horizontal.png'
            alt=''
            className='h-auto w-44 object-contain dark:hidden'
          />
          <img
            src='/jdtoken/logo-horizontal-white.png'
            alt=''
            className='hidden h-auto w-44 object-contain dark:block'
          />
        </Link>
        <div className='text-muted-foreground min-w-0 space-y-2 text-xs leading-relaxed'>
          <p>
            &copy; {new Date().getFullYear()} 及第嘉序(北京)智算科技有限公司.{' '}
            {t('footer.defaultCopyright')}
          </p>
          <div className='flex flex-wrap items-center gap-x-4 gap-y-2'>
            <a
              href='https://beian.miit.gov.cn/'
              target='_blank'
              rel='noopener noreferrer'
              className='hover:text-foreground'
            >
              京ICP备2026065372号-1
            </a>
            <a
              href='https://beian.mps.gov.cn/#/query/webSearch?code=11010602203425'
              target='_blank'
              rel='noopener noreferrer'
              className='hover:text-foreground inline-flex items-center gap-1.5'
            >
              <img
                src='/jdtoken/police-beian.png'
                alt=''
                width={18}
                height={20}
                className='shrink-0'
              />
              京公网安备11010602203425号
            </a>
            {status?.user_agreement_enabled && (
              <Link to='/user-agreement' className='hover:text-foreground'>
                {t('User Agreement')}
              </Link>
            )}
            {status?.privacy_policy_enabled && (
              <Link to='/privacy-policy' className='hover:text-foreground'>
                {t('Privacy Policy')}
              </Link>
            )}
          </div>
        </div>
      </div>
    </footer>
  )
}
