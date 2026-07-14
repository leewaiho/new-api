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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, RefreshCcw, Wrench, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

import {
  applyToolCompatibilityEventSuggestion,
  getToolCompatibilityEvents,
  restoreToolCompatibilityEventModelDefault,
  updateToolCompatibilityEventStatus,
} from '../api'
import type { ToolCompatibilityEvent } from '../types'

type ToolCompatibilityEventsProps = {
  channelId?: number
  channelModels?: string[]
}

const formatTime = (timestamp: number) =>
  timestamp > 0 ? new Date(timestamp * 1000).toLocaleString() : '-'

const modelForEvent = (event: ToolCompatibilityEvent) =>
  event.requested_model || event.upstream_model || '-'

export function ToolCompatibilityEvents({
  channelId,
  channelModels = [],
}: ToolCompatibilityEventsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const queryKey = ['tool-compatibility-events', channelId]
  const eventsQuery = useQuery({
    queryKey,
    queryFn: () =>
      getToolCompatibilityEvents({ channel_id: channelId, page_size: 30 }),
    enabled: Boolean(channelId),
  })
  const refresh = () => queryClient.invalidateQueries({ queryKey })
  const mutation = useMutation({
    mutationFn: async (input: {
      event: ToolCompatibilityEvent
      action: 'apply' | 'restore' | 'ignore'
    }) => {
      if (input.action === 'apply') {
        return applyToolCompatibilityEventSuggestion(input.event.id)
      }
      if (input.action === 'restore') {
        return restoreToolCompatibilityEventModelDefault(input.event.id)
      }
      return updateToolCompatibilityEventStatus(input.event.id, 'ignored')
    },
    onSuccess: (_, input) => {
      let message = t('Compatibility issue ignored')
      if (input.action === 'apply') {
        message = t('Compatibility suggestion applied')
      } else if (input.action === 'restore') {
        message = t('Model default restored')
      }
      toast.success(message)
      refresh()
    },
    onError: (error: Error) =>
      toast.error(error.message || t('Operation failed')),
  })

  if (!channelId) {
    return (
      <Alert>
        <AlertDescription>
          {t('Save this channel before viewing compatibility issues.')}
        </AlertDescription>
      </Alert>
    )
  }

  const events = eventsQuery.data?.data || []
  return (
    <section className='border-border space-y-3 rounded-md border p-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <h4 className='text-sm font-medium'>{t('Compatibility Issues')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Only sanitized upstream errors and tool policy decisions are shown.'
            )}
          </p>
        </div>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={refresh}
          disabled={eventsQuery.isFetching}
        >
          <RefreshCcw className='mr-1 h-3.5 w-3.5' />
          {t('Refresh')}
        </Button>
      </div>
      {channelModels.length > 0 ? (
        <p className='text-muted-foreground text-xs'>
          {t('Suggestions change only the event model by default.')}:{' '}
          {channelModels.join(', ')}
        </p>
      ) : null}
      {eventsQuery.isLoading ? (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      ) : null}
      {!eventsQuery.isLoading && events.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('No compatibility issues recorded for this channel.')}
        </p>
      ) : null}
      <div className='space-y-2'>
        {events.map((event) => {
          const hasSuggestion = Boolean(event.suggested_policy)
          const isOpen = event.resolution_status === 'open'
          return (
            <article
              key={event.id}
              className='bg-muted/25 space-y-2 rounded-md p-3 text-xs'
            >
              <div className='flex flex-wrap items-center gap-1.5'>
                <Badge variant={isOpen ? 'destructive' : 'secondary'}>
                  {event.event_type}
                </Badge>
                <Badge variant='outline'>{event.resolution_status}</Badge>
                <span className='font-medium'>{event.route}</span>
                <span>{modelForEvent(event)}</span>
                <span>
                  {event.tool_type}
                  {event.tool_name ? ` / ${event.tool_name}` : ''}
                </span>
              </div>
              <div className='text-muted-foreground grid gap-1 sm:grid-cols-2'>
                <span>
                  {t('Count')}: {event.occurrence_count}
                </span>
                <span>
                  {t('Last seen')}: {formatTime(event.last_seen_at)}
                </span>
                <span>
                  {t('Current policy')}: {event.current_policy || '-'}
                </span>
                <span>
                  {t('Suggested policy')}: {event.suggested_policy || '-'}
                </span>
              </div>
              {event.sanitized_error ? (
                <p className='text-muted-foreground break-all'>
                  {event.sanitized_error}
                </p>
              ) : null}
              {isOpen ? (
                <div className='flex flex-wrap gap-2'>
                  {hasSuggestion ? (
                    <Button
                      type='button'
                      size='sm'
                      onClick={() =>
                        mutation.mutate({ event, action: 'apply' })
                      }
                      disabled={mutation.isPending}
                    >
                      <Wrench className='mr-1 h-3.5 w-3.5' />
                      {t('Apply suggestion')}
                    </Button>
                  ) : null}
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() => mutation.mutate({ event, action: 'ignore' })}
                    disabled={mutation.isPending}
                  >
                    <X className='mr-1 h-3.5 w-3.5' />
                    {t('Ignore')}
                  </Button>
                  <Button
                    type='button'
                    size='sm'
                    variant='ghost'
                    onClick={() =>
                      mutation.mutate({ event, action: 'restore' })
                    }
                    disabled={mutation.isPending}
                  >
                    <Check className='mr-1 h-3.5 w-3.5' />
                    {t('Restore model default')}
                  </Button>
                </div>
              ) : null}
              <Separator />
            </article>
          )
        })}
      </div>
    </section>
  )
}
