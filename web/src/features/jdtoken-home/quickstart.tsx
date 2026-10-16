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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'

import styles from './home.module.css'

export function Quickstart(props: { model?: string }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const configuredAddress =
    typeof status?.server_address === 'string'
      ? status.server_address.trim()
      : ''
  // Match the API-key screen's configured gateway; fall back to the current host.
  const baseUrl = `${(configuredAddress || window.location.origin).replace(/\/+$/, '')}/v1`
  const code = [
    'import os',
    'from openai import OpenAI',
    '',
    'client = OpenAI(',
    '    api_key=os.environ["OPENAI_API_KEY"],',
    `    base_url=${JSON.stringify(baseUrl)},`,
    ')',
    '',
    'response = client.chat.completions.create(',
    `    model=${JSON.stringify(props.model ?? 'YOUR_MODEL_ID')},`,
    '    messages=[{"role": "user", "content": "Hello!"}],',
    ')',
    'print(response.choices[0].message.content)',
  ].join('\n')

  return (
    <section
      className={styles.softSection}
      id='quickstart'
      aria-labelledby='jdtoken-quickstart-title'
    >
      <div className={styles.container}>
        <div className={styles.sectionHeading}>
          <p className={styles.eyebrow}>{t('Connect with familiar tools')}</p>
          <h2 id='jdtoken-quickstart-title'>
            {t('Connect with the SDK you already use')}
          </h2>
          <p>{t('Create an API key, select a model, and start building.')}</p>
        </div>
        <div className={styles.codeExample}>
          <CodeBlock
            code={code}
            language='python'
            filename='quickstart.py'
            title='quickstart.py'
            showToolbar
            enableCollapse={false}
          >
            <CodeBlockCopyButton />
          </CodeBlock>
          <p>
            {t(
              'Install the openai package and set OPENAI_API_KEY to your own key.'
            )}
          </p>
          {!props.model && (
            <p>
              {t(
                'Replace YOUR_MODEL_ID with an available chat model from Model Square.'
              )}
            </p>
          )}
          <Button role='link' variant='outline' render={<Link to='/keys' />}>
            {t('Manage API keys')} <ArrowRight aria-hidden='true' />
          </Button>
        </div>
      </div>
    </section>
  )
}
