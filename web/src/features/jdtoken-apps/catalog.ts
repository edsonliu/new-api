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
import type { TFunction } from 'i18next'
import {
  Bot,
  Clapperboard,
  Images,
  MessageSquare,
  WandSparkles,
} from 'lucide-react'

export type Application = {
  id: string
  title: string
  description: string
  tags: string[]
  icon: typeof Images
  destination?: '/playground'
}

export function getApplications(t: TFunction): Application[] {
  return [
    {
      id: 'images',
      title: t('Image studio'),
      icon: Images,
      tags: [t('Text to image'), t('Image to image'), t('Image editing')],
      description: t(
        'Turn ideas and reference images into visual content. The image creation workspace is coming soon.'
      ),
    },
    {
      id: 'video',
      title: t('Video studio'),
      icon: Clapperboard,
      tags: [t('Text to video'), t('Image to video'), t('Reference materials')],
      description: t(
        'Create videos from text, images and audio references. The visual workspace is coming soon; video generation is already available through the API.'
      ),
    },
    {
      id: 'prompts',
      title: t('Prompt assistant'),
      icon: WandSparkles,
      tags: [
        t('Prompt optimization'),
        t('Structured prompts'),
        t('Creative writing'),
      ],
      description: t(
        'Shape a rough idea into a clear, structured prompt for writing, imagery or video. This dedicated assistant is coming soon.'
      ),
    },
    {
      id: 'chat',
      title: t('AI conversation'),
      icon: MessageSquare,
      tags: [t('Multi-model chat'), t('Writing'), t('Code assistance')],
      description: t(
        'Explore the models available to your account in the playground. Ask questions, work on copy and get help with code.'
      ),
      destination: '/playground',
    },
    {
      id: 'materials',
      title: t('Creative material library'),
      icon: Images,
      tags: [t('Images'), t('Video'), t('Audio')],
      description: t(
        'Reuse image, video and audio materials in Seedance generation. The library interface is coming soon; material management is available through the API.'
      ),
    },
    {
      id: 'agents',
      title: t('Agent workspace'),
      icon: Bot,
      tags: [
        t('Task collaboration'),
        t('Creative workflows'),
        t('Human review'),
      ],
      description: t(
        'A workspace for task-focused AI assistants and creative workflows. More applications will be added here as they become available.'
      ),
    },
  ]
}
