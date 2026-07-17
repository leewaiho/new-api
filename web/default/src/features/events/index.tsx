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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ToolCompatibilityEvents } from '@/features/channels/components/tool-compatibility-events'

export function Events() {
  const { t } = useTranslation()

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Events')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='tool-compatibility' className='h-full'>
          <TabsList>
            <TabsTrigger value='tool-compatibility'>
              {t('Tool compatibility events')}
            </TabsTrigger>
          </TabsList>
          <TabsContent
            value='tool-compatibility'
            className='mt-4 min-h-0 flex-1 overflow-y-auto'
          >
            <ToolCompatibilityEvents mode='global' />
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
