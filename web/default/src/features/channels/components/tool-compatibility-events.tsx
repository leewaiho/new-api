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
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { type ColumnDef } from '@tanstack/react-table'
import {
  AlertTriangle,
  Check,
  ChevronDown,
  CircleCheck,
  CircleMinus,
  RefreshCcw,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
  Wrench,
  X,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  applyToolCompatibilityEventSuggestion,
  getToolCompatibilityEventFilterOptions,
  getToolCompatibilityEvents,
  restoreToolCompatibilityEventModelDefault,
  updateToolCompatibilityEventStatus,
} from '../api'
import {
  ADVANCED_CUSTOM_RESPONSES_TOOL_TYPES,
  normalizeAdvancedCustomResponsesToolType,
  resolveAdvancedCustomResponsesToolPolicy,
} from '../lib/advanced-custom'
import type {
  AdvancedCustomRoute,
  ToolCompatibilityEvent,
  ToolCompatibilityMutationResult,
  ToolCompatibilityResolutionStatus,
} from '../types'

type ToolCompatibilityEventsProps = {
  mode?: 'channel' | 'global'
  channelId?: number
  channelModels?: string[]
  routes?: AdvancedCustomRoute[]
  onRouteConfigChange?: (route: AdvancedCustomRoute) => void
}

type CompatibilityMutationAction =
  | 'apply'
  | 'restore'
  | 'ignore'
  | 'resolve'
  | 'apply-route'
  | 'restore-route'

type RouteConfirmation = {
  event: ToolCompatibilityEvent
  action: 'apply-route' | 'restore-route'
}

const eventPageSize = 50
const positiveEventTypes = new Set(['accepted_definition', 'invoked'])

type GlobalEventFilters = {
  channelId: string
  model: string
  resolutionStatus: 'all' | ToolCompatibilityResolutionStatus
  route: string
}

const defaultGlobalEventFilters: GlobalEventFilters = {
  channelId: '',
  model: '',
  resolutionStatus: 'all',
  route: '',
}

type EventPresentation = {
  label: string
  icon: LucideIcon
  variant: 'destructive' | 'secondary'
  className?: string
}

function getEventPresentation(
  event: ToolCompatibilityEvent
): EventPresentation {
  if (event.resolution_status === 'resolved') {
    return {
      label: 'Handled',
      icon: CircleCheck,
      variant: 'secondary',
      className:
        'bg-emerald-500/10 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300',
    }
  }
  if (event.resolution_status === 'ignored') {
    return {
      label: 'Ignored',
      icon: CircleMinus,
      variant: 'secondary',
      className: 'text-muted-foreground',
    }
  }
  if (
    positiveEventTypes.has(event.event_type) ||
    (event.event_type === 'name_conflict' &&
      event.current_policy === 'deduplicate') ||
    (event.event_type === 'policy_drop' && event.current_policy === 'drop')
  ) {
    return {
      label: 'Expected policy',
      icon: ShieldCheck,
      variant: 'secondary',
      className:
        'bg-sky-500/10 text-sky-700 dark:bg-sky-500/20 dark:text-sky-300',
    }
  }
  return {
    label: 'Needs attention',
    icon: AlertTriangle,
    variant: 'destructive',
  }
}

const formatTime = (timestamp: number) =>
  timestamp > 0 ? new Date(timestamp * 1000).toLocaleString() : '-'

const modelForEvent = (event: ToolCompatibilityEvent) =>
  event.requested_model || event.upstream_model || ''

function eventMatchesModel(event: ToolCompatibilityEvent, model: string) {
  return event.requested_model === model || event.upstream_model === model
}

function latestEvent(
  events: ToolCompatibilityEvent[],
  predicate: (event: ToolCompatibilityEvent) => boolean
) {
  return events.find(predicate)
}

export function ToolCompatibilityEvents({
  mode = 'channel',
  channelId,
  channelModels = [],
  routes = [],
  onRouteConfigChange,
}: ToolCompatibilityEventsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isRoot = useAuthStore(
    (state) => state.auth.user?.role === ROLE.SUPER_ADMIN
  )
  const [targetModels, setTargetModels] = useState<Record<number, string>>({})
  const [routeConfirmation, setRouteConfirmation] =
    useState<RouteConfirmation | null>(null)
  const [globalFilters, setGlobalFilters] = useState<GlobalEventFilters>(
    defaultGlobalEventFilters
  )
  const [globalFilterDraft, setGlobalFilterDraft] =
    useState<GlobalEventFilters>(defaultGlobalEventFilters)
  const [globalPagination, setGlobalPagination] = useState({
    pageIndex: 0,
    pageSize: eventPageSize,
  })
  const isGlobal = mode === 'global'
  const channelQueryKey = ['tool-compatibility-events', 'channel', channelId]
  const globalQueryKey = [
    'tool-compatibility-events',
    'global',
    globalFilters,
    globalPagination.pageIndex,
    globalPagination.pageSize,
  ]
  const eventsQuery = useInfiniteQuery({
    queryKey: channelQueryKey,
    initialPageParam: 1,
    queryFn: ({ pageParam }) =>
      getToolCompatibilityEvents({
        channel_id: channelId,
        page: pageParam,
        page_size: eventPageSize,
      }),
    enabled: !isGlobal && Boolean(channelId),
    getNextPageParam: (lastPage) => {
      const loaded = lastPage.page * lastPage.page_size
      return loaded < lastPage.total ? lastPage.page + 1 : undefined
    },
  })
  const globalEventsQuery = useQuery({
    queryKey: globalQueryKey,
    queryFn: () =>
      getToolCompatibilityEvents({
        ...(globalFilters.channelId
          ? { channel_id: Number(globalFilters.channelId) }
          : {}),
        ...(globalFilters.model ? { model: globalFilters.model } : {}),
        ...(globalFilters.route ? { route: globalFilters.route } : {}),
        ...(globalFilters.resolutionStatus === 'all'
          ? {}
          : { resolution_status: globalFilters.resolutionStatus }),
        page: globalPagination.pageIndex + 1,
        page_size: globalPagination.pageSize,
      }),
    enabled: isGlobal,
    placeholderData: (previousData) => previousData,
  })
  const globalEventFilterOptionsQuery = useQuery({
    queryKey: ['tool-compatibility-event-filter-options'],
    queryFn: getToolCompatibilityEventFilterOptions,
    enabled: isGlobal,
    staleTime: 60_000,
  })

  const refresh = () =>
    queryClient.invalidateQueries({
      queryKey: isGlobal ? globalQueryKey : channelQueryKey,
    })
  const events = useMemo(
    () => eventsQuery.data?.pages.flatMap((page) => page.data || []) || [],
    [eventsQuery.data]
  )
  const total = eventsQuery.data?.pages[0]?.total || 0
  const responseRoutes = useMemo(
    () =>
      routes.filter(
        (route) =>
          route.incoming_path === '/v1/responses' ||
          route.incoming_path === '/v1/responses/compact'
      ),
    [routes]
  )
  const matrixModels = useMemo(
    () => [
      ...new Set([
        ...channelModels.map((model) => model.trim()).filter(Boolean),
        ...responseRoutes.flatMap((route) =>
          (
            route.converter_options?.responses_tool_model_overrides || []
          ).flatMap((override) => override.models || [])
        ),
        ...events.map(modelForEvent).filter(Boolean),
      ]),
    ],
    [channelModels, events, responseRoutes]
  )
  const configuredModelSet = useMemo(
    () => new Set(channelModels.map((model) => model.trim()).filter(Boolean)),
    [channelModels]
  )

  const mutation = useMutation({
    mutationFn: async (input: {
      event: ToolCompatibilityEvent
      action: CompatibilityMutationAction
    }) => {
      const targetModel = targetModels[input.event.id]?.trim()
      if (input.action === 'apply') {
        return applyToolCompatibilityEventSuggestion(input.event.id, {
          target_model: targetModel || undefined,
          scope: 'model',
        })
      }
      if (input.action === 'restore') {
        return restoreToolCompatibilityEventModelDefault(input.event.id, {
          target_model: targetModel || undefined,
          scope: 'model',
        })
      }
      if (input.action === 'apply-route') {
        return applyToolCompatibilityEventSuggestion(input.event.id, {
          scope: 'route',
          confirm_route: true,
        })
      }
      if (input.action === 'restore-route') {
        return restoreToolCompatibilityEventModelDefault(input.event.id, {
          scope: 'route',
          confirm_route: true,
        })
      }
      const status: ToolCompatibilityResolutionStatus =
        input.action === 'ignore' ? 'ignored' : 'resolved'
      return updateToolCompatibilityEventStatus(input.event.id, status)
    },
    onSuccess: (response, input) => {
      if (
        input.action === 'apply' ||
        input.action === 'restore' ||
        input.action === 'apply-route' ||
        input.action === 'restore-route'
      ) {
        const mutationResult = response.data as
          | ToolCompatibilityMutationResult
          | undefined
        if (mutationResult?.route_config) {
          onRouteConfigChange?.(mutationResult.route_config)
        }
      }
      const messages: Record<CompatibilityMutationAction, string> = {
        apply: 'Compatibility suggestion applied',
        restore: 'Model default restored',
        ignore: 'Compatibility issue ignored',
        resolve: 'Compatibility issue resolved',
        'apply-route': 'Route compatibility suggestion applied',
        'restore-route': 'Route safe defaults restored',
      }
      toast.success(t(messages[input.action]))
      setRouteConfirmation(null)
      refresh()
    },
    onError: (error: Error) =>
      toast.error(error.message || t('Operation failed')),
  })

  const globalEvents = globalEventsQuery.data?.data || []
  const globalTotal = globalEventsQuery.data?.total || 0
  const triggerGlobalEventAction = (
    event: ToolCompatibilityEvent,
    action: CompatibilityMutationAction
  ) => {
    if (
      (action === 'apply' || action === 'restore-route') &&
      event.event_type === 'name_conflict'
    ) {
      setRouteConfirmation({
        event,
        action: action === 'apply' ? 'apply-route' : 'restore-route',
      })
      return
    }
    mutation.mutate({ event, action })
  }
  const globalColumns = useMemo<ColumnDef<ToolCompatibilityEvent>[]>(
    () => [
      {
        id: 'status',
        header: t('Status'),
        cell: ({ row }) => {
          const presentation = getEventPresentation(row.original)
          const StatusIcon = presentation.icon
          return (
            <Badge
              variant={presentation.variant}
              className={presentation.className}
            >
              <StatusIcon data-icon='inline-start' aria-hidden='true' />
              {t(presentation.label)}
            </Badge>
          )
        },
      },
      {
        accessorKey: 'event_type',
        header: t('Event type'),
        cell: ({ row }) => (
          <Badge variant='outline'>{t(row.original.event_type)}</Badge>
        ),
      },
      {
        id: 'channel',
        header: t('Channel'),
        cell: ({ row }) =>
          row.original.channel_name || `#${row.original.channel_id}`,
      },
      {
        accessorKey: 'route',
        header: t('Route'),
        cell: ({ row }) => (
          <span className='font-medium break-all'>{row.original.route}</span>
        ),
      },
      {
        id: 'model',
        header: t('Model'),
        cell: ({ row }) => (
          <span className='break-all'>
            {modelForEvent(row.original) || '-'}
          </span>
        ),
      },
      {
        id: 'tool',
        header: t('Tool'),
        cell: ({ row }) => (
          <span className='break-all'>
            {row.original.tool_type || '-'}
            {row.original.tool_name ? ` / ${row.original.tool_name}` : ''}
          </span>
        ),
      },
      {
        id: 'policy',
        header: t('Policy'),
        cell: ({ row }) => (
          <div className='space-y-1 whitespace-nowrap'>
            <div>
              {t('Current policy')}: {row.original.current_policy || '-'}
            </div>
            <div>
              {t('Suggested policy')}: {row.original.suggested_policy || '-'}
            </div>
          </div>
        ),
      },
      {
        accessorKey: 'occurrence_count',
        header: t('Count'),
      },
      {
        accessorKey: 'last_seen_at',
        header: t('Last seen'),
        cell: ({ row }) => formatTime(row.original.last_seen_at),
      },
      {
        id: 'actions',
        header: t('Actions'),
        size: 280,
        minSize: 240,
        cell: ({ row }) => {
          const event = row.original
          const isConflict = event.event_type === 'name_conflict'
          const hasSuggestion = Boolean(event.suggested_policy)
          return (
            <div className='flex w-[280px] flex-wrap gap-1'>
              {hasSuggestion ? (
                <Button
                  type='button'
                  size='sm'
                  onClick={() => triggerGlobalEventAction(event, 'apply')}
                  disabled={mutation.isPending}
                >
                  <Wrench className='mr-1 h-3.5 w-3.5' />
                  {isConflict
                    ? t('Apply to entire route')
                    : t('Apply to selected model')}
                </Button>
              ) : null}
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => triggerGlobalEventAction(event, 'ignore')}
                disabled={mutation.isPending}
              >
                <X className='mr-1 h-3.5 w-3.5' />
                {t('Ignore')}
              </Button>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => triggerGlobalEventAction(event, 'resolve')}
                disabled={mutation.isPending}
              >
                <Check className='mr-1 h-3.5 w-3.5' />
                {t('Mark resolved')}
              </Button>
              {isConflict ? (
                <Button
                  type='button'
                  size='sm'
                  variant='ghost'
                  onClick={() =>
                    triggerGlobalEventAction(event, 'restore-route')
                  }
                  disabled={mutation.isPending}
                >
                  <RotateCcw className='mr-1 h-3.5 w-3.5' />
                  {t('Restore route safe defaults')}
                </Button>
              ) : (
                <Button
                  type='button'
                  size='sm'
                  variant='ghost'
                  onClick={() => triggerGlobalEventAction(event, 'restore')}
                  disabled={mutation.isPending}
                >
                  <RotateCcw className='mr-1 h-3.5 w-3.5' />
                  {t('Restore selected model default')}
                </Button>
              )}
            </div>
          )
        },
      },
    ],
    [mutation.isPending, t]
  )
  const { table: globalTable } = useDataTable({
    data: globalEvents,
    columns: globalColumns,
    pagination: globalPagination,
    onPaginationChange: setGlobalPagination,
    manualPagination: true,
    manualFiltering: true,
    totalCount: globalTotal,
    enableRowSelection: false,
  })

  if (isGlobal) {
    return (
      <>
        {!isRoot ? (
          <Alert>
            <ShieldAlert className='h-4 w-4' />
            <AlertDescription>
              {t('Compatibility configuration changes require a root account.')}
            </AlertDescription>
          </Alert>
        ) : null}
        {globalEventsQuery.isError ? (
          <Alert variant='destructive'>
            <AlertTriangle className='h-4 w-4' />
            <AlertDescription>
              {globalEventsQuery.error instanceof Error
                ? globalEventsQuery.error.message
                : t('Failed to load compatibility issues.')}
            </AlertDescription>
          </Alert>
        ) : null}
        <DataTablePage
          table={globalTable}
          columns={globalColumns}
          isLoading={globalEventsQuery.isLoading}
          isFetching={globalEventsQuery.isFetching}
          emptyTitle={t('No compatibility issues recorded.')}
          emptyDescription={t(
            'Tool compatibility events will appear here after matching requests are processed.'
          )}
          skeletonKeyPrefix='tool-compatibility-event-skeleton'
          applyHeaderSize
          tableClassName='[&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:align-top [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_th]:text-[13px]'
          pinnedColumns={[{ columnId: 'actions', side: 'right' }]}
          toolbar={
            <LogsFilterToolbar
              table={globalTable}
              primaryFilters={
                <>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: 'all', label: t('All statuses') },
                        { value: 'open', label: t('Unresolved') },
                        { value: 'resolved', label: t('Resolved') },
                        { value: 'ignored', label: t('Ignored') },
                      ]}
                      value={globalFilterDraft.resolutionStatus}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          resolutionStatus: (value ?? 'all') as
                            | 'all'
                            | ToolCompatibilityResolutionStatus,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='all'>{t('All statuses')}</SelectItem>
                        <SelectItem value='open'>{t('Unresolved')}</SelectItem>
                        <SelectItem value='resolved'>
                          {t('Resolved')}
                        </SelectItem>
                        <SelectItem value='ignored'>{t('Ignored')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All channels') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.channels ||
                          []
                        ).map((channel) => ({
                          value: String(channel.id),
                          label: channel.name || `#${channel.id}`,
                        })),
                      ]}
                      value={globalFilterDraft.channelId || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          channelId: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>
                          {t('All channels')}
                        </SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.channels ||
                          []
                        ).map((channel) => (
                          <SelectItem
                            key={channel.id}
                            value={String(channel.id)}
                          >
                            {channel.name || `#${channel.id}`}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All models') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.models || []
                        ).map((model) => ({ value: model, label: model })),
                      ]}
                      value={globalFilterDraft.model || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          model: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>{t('All models')}</SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.models || []
                        ).map((model) => (
                          <SelectItem key={model} value={model}>
                            {model}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All routes') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.routes || []
                        ).map((route) => ({ value: route, label: route })),
                      ]}
                      value={globalFilterDraft.route || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          route: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>{t('All routes')}</SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.routes || []
                        ).map((route) => (
                          <SelectItem key={route} value={route}>
                            {route}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                </>
              }
              mobilePinnedFilters={
                <LogsFilterField>
                  <Select
                    items={[
                      { value: 'all', label: t('All statuses') },
                      { value: 'open', label: t('Unresolved') },
                      { value: 'resolved', label: t('Resolved') },
                      { value: 'ignored', label: t('Ignored') },
                    ]}
                    value={globalFilterDraft.resolutionStatus}
                    onValueChange={(value) =>
                      setGlobalFilterDraft((current) => ({
                        ...current,
                        resolutionStatus: (value ?? 'all') as
                          | 'all'
                          | ToolCompatibilityResolutionStatus,
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='all'>{t('All statuses')}</SelectItem>
                      <SelectItem value='open'>{t('Unresolved')}</SelectItem>
                      <SelectItem value='resolved'>{t('Resolved')}</SelectItem>
                      <SelectItem value='ignored'>{t('Ignored')}</SelectItem>
                    </SelectContent>
                  </Select>
                </LogsFilterField>
              }
              mobileFilters={
                <>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All channels') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.channels ||
                          []
                        ).map((channel) => ({
                          value: String(channel.id),
                          label: channel.name || `#${channel.id}`,
                        })),
                      ]}
                      value={globalFilterDraft.channelId || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          channelId: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>
                          {t('All channels')}
                        </SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.channels ||
                          []
                        ).map((channel) => (
                          <SelectItem
                            key={channel.id}
                            value={String(channel.id)}
                          >
                            {channel.name || `#${channel.id}`}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All models') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.models || []
                        ).map((model) => ({ value: model, label: model })),
                      ]}
                      value={globalFilterDraft.model || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          model: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>{t('All models')}</SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.models || []
                        ).map((model) => (
                          <SelectItem key={model} value={model}>
                            {model}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                  <LogsFilterField>
                    <Select
                      items={[
                        { value: '__all', label: t('All routes') },
                        ...(
                          globalEventFilterOptionsQuery.data?.data.routes || []
                        ).map((route) => ({ value: route, label: route })),
                      ]}
                      value={globalFilterDraft.route || '__all'}
                      onValueChange={(value) =>
                        setGlobalFilterDraft((current) => ({
                          ...current,
                          route: !value || value === '__all' ? '' : value,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='__all'>{t('All routes')}</SelectItem>
                        {(
                          globalEventFilterOptionsQuery.data?.data.routes || []
                        ).map((route) => (
                          <SelectItem key={route} value={route}>
                            {route}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </LogsFilterField>
                </>
              }
              mobileFilterCount={
                [
                  globalFilterDraft.channelId,
                  globalFilterDraft.model,
                  globalFilterDraft.route,
                ].filter(Boolean).length
              }
              stats={
                <span className='text-muted-foreground text-xs'>
                  {t('{{loaded}} of {{total}} loaded', {
                    loaded: globalEvents.length,
                    total: globalTotal,
                  })}
                </span>
              }
              actionStart={
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={refresh}
                  disabled={globalEventsQuery.isFetching}
                >
                  <RefreshCcw className='mr-1 h-3.5 w-3.5' />
                  {t('Refresh')}
                </Button>
              }
              hasActiveFilters={
                globalFilterDraft.channelId !== '' ||
                globalFilterDraft.model !== '' ||
                globalFilterDraft.route !== '' ||
                globalFilterDraft.resolutionStatus !== 'all'
              }
              searchLoading={globalEventsQuery.isFetching}
              onReset={() => {
                setGlobalFilterDraft(defaultGlobalEventFilters)
                setGlobalFilters(defaultGlobalEventFilters)
                setGlobalPagination((current) => ({
                  ...current,
                  pageIndex: 0,
                }))
              }}
              onSearch={() => {
                setGlobalFilters(globalFilterDraft)
                setGlobalPagination((current) => ({
                  ...current,
                  pageIndex: 0,
                }))
              }}
            />
          }
        />

        <AlertDialog
          open={Boolean(routeConfirmation)}
          onOpenChange={(open) => {
            if (!open) setRouteConfirmation(null)
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t('Confirm route-wide change')}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t(
                  'This changes tool handling for every model using route {{route}}. Model-specific overrides remain unchanged.',
                  { route: routeConfirmation?.event.route || '' }
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => {
                  if (!routeConfirmation) return
                  mutation.mutate(routeConfirmation)
                }}
              >
                {t('Confirm route-wide change')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </>
    )
  }

  if (!isGlobal && !channelId) {
    return (
      <Alert>
        <AlertDescription>
          {t('Save this channel before viewing compatibility issues.')}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <section className='border-border space-y-4 rounded-md border p-3'>
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

      {!isRoot ? (
        <Alert>
          <ShieldAlert className='h-4 w-4' />
          <AlertDescription>
            {t('Compatibility configuration changes require a root account.')}
          </AlertDescription>
        </Alert>
      ) : null}
      {eventsQuery.isError ? (
        <Alert variant='destructive'>
          <AlertTriangle className='h-4 w-4' />
          <AlertDescription>
            {eventsQuery.error instanceof Error
              ? eventsQuery.error.message
              : t('Failed to load compatibility issues.')}
          </AlertDescription>
        </Alert>
      ) : null}

      {!isGlobal ? (
        <>
          <CapabilityMatrix
            routes={responseRoutes}
            models={matrixModels}
            configuredModelSet={configuredModelSet}
            events={events}
          />
          <Separator />
        </>
      ) : null}
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h5 className='text-sm font-medium'>{t('Recorded issues')}</h5>
        <span className='text-muted-foreground text-xs'>
          {t('{{loaded}} of {{total}} loaded', {
            loaded: events.length,
            total,
          })}
        </span>
      </div>
      {eventsQuery.isLoading ? (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      ) : null}
      {!eventsQuery.isLoading && !eventsQuery.isError && events.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t(
            isGlobal
              ? 'No compatibility issues recorded.'
              : 'No compatibility issues recorded for this channel.'
          )}
        </p>
      ) : null}
      <div className='space-y-2'>
        {events.map((event) => {
          const hasSuggestion = Boolean(event.suggested_policy)
          const eventModel = modelForEvent(event)
          const modelOptions = [
            ...new Set(
              [
                ...channelModels,
                ...(isGlobal && eventModel ? [eventModel] : []),
              ]
                .map((model) => model.trim())
                .filter(Boolean)
            ),
          ]
          const selectedModel =
            targetModels[event.id] ||
            (modelOptions.includes(eventModel)
              ? eventModel
              : modelOptions[0] || '')
          const isConflict = event.event_type === 'name_conflict'
          const presentation = getEventPresentation(event)
          const EventIcon = presentation.icon
          const isFirstEventForRoute =
            events.find((candidate) => candidate.route === event.route)?.id ===
            event.id
          return (
            <article
              key={event.id}
              className='bg-muted/25 space-y-2 rounded-md p-3 text-xs'
            >
              <div className='flex flex-wrap items-center gap-1.5'>
                <Badge
                  variant={presentation.variant}
                  className={presentation.className}
                >
                  <EventIcon data-icon='inline-start' aria-hidden='true' />
                  {t(presentation.label)}
                </Badge>
                <Badge variant='outline'>{t(event.event_type)}</Badge>
                {isGlobal ? (
                  <Badge variant='outline'>
                    {t('Channel {{name}}', {
                      name: event.channel_name || `#${event.channel_id}`,
                    })}
                  </Badge>
                ) : null}
                <span className='font-medium break-all'>{event.route}</span>
                <span className='break-all'>{eventModel || '-'}</span>
                <span className='break-all'>
                  {event.tool_type || '-'}
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
              {isRoot ? (
                <div className='space-y-2'>
                  {!isConflict && modelOptions.length > 0 ? (
                    <Select
                      value={selectedModel}
                      onValueChange={(model) => {
                        if (!model) return
                        setTargetModels((current) => ({
                          ...current,
                          [event.id]: model,
                        }))
                      }}
                    >
                      <SelectTrigger className='w-full sm:w-72'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {modelOptions.map((model) => (
                          <SelectItem key={model} value={model}>
                            {model}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : null}
                  <div className='flex flex-wrap gap-2'>
                    {hasSuggestion ? (
                      <Button
                        type='button'
                        size='sm'
                        onClick={() => {
                          if (isConflict) {
                            setRouteConfirmation({
                              event,
                              action: 'apply-route',
                            })
                            return
                          }
                          mutation.mutate({ event, action: 'apply' })
                        }}
                        disabled={
                          mutation.isPending || (!isConflict && !selectedModel)
                        }
                      >
                        <Wrench className='mr-1 h-3.5 w-3.5' />
                        {isConflict
                          ? t('Apply to entire route')
                          : t('Apply to selected model')}
                      </Button>
                    ) : null}
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={() =>
                        mutation.mutate({ event, action: 'ignore' })
                      }
                      disabled={mutation.isPending}
                    >
                      <X className='mr-1 h-3.5 w-3.5' />
                      {t('Ignore')}
                    </Button>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={() =>
                        mutation.mutate({ event, action: 'resolve' })
                      }
                      disabled={mutation.isPending}
                    >
                      <Check className='mr-1 h-3.5 w-3.5' />
                      {t('Mark resolved')}
                    </Button>
                    {!isConflict ? (
                      <Button
                        type='button'
                        size='sm'
                        variant='ghost'
                        onClick={() =>
                          mutation.mutate({ event, action: 'restore' })
                        }
                        disabled={mutation.isPending}
                      >
                        <RotateCcw className='mr-1 h-3.5 w-3.5' />
                        {t('Restore selected model default')}
                      </Button>
                    ) : null}
                    {isFirstEventForRoute ? (
                      <Button
                        type='button'
                        size='sm'
                        variant='ghost'
                        onClick={() =>
                          setRouteConfirmation({
                            event,
                            action: 'restore-route',
                          })
                        }
                        disabled={mutation.isPending}
                      >
                        <RotateCcw className='mr-1 h-3.5 w-3.5' />
                        {t('Restore route safe defaults')}
                      </Button>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </article>
          )
        })}
      </div>
      {eventsQuery.hasNextPage ? (
        <Button
          type='button'
          variant='outline'
          className='w-full'
          onClick={() => eventsQuery.fetchNextPage()}
          disabled={eventsQuery.isFetchingNextPage}
        >
          <ChevronDown className='mr-1 h-4 w-4' />
          {eventsQuery.isFetchingNextPage ? t('Loading...') : t('Load more')}
        </Button>
      ) : null}

      <AlertDialog
        open={Boolean(routeConfirmation)}
        onOpenChange={(open) => {
          if (!open) setRouteConfirmation(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Confirm route-wide change')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This changes tool handling for every model using route {{route}}. Model-specific overrides remain unchanged.',
                { route: routeConfirmation?.event.route || '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (!routeConfirmation) return
                mutation.mutate(routeConfirmation)
              }}
            >
              {t('Confirm route-wide change')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}

function CapabilityMatrix({
  routes,
  models,
  configuredModelSet,
  events,
}: {
  routes: AdvancedCustomRoute[]
  models: string[]
  configuredModelSet: Set<string>
  events: ToolCompatibilityEvent[]
}) {
  const { t } = useTranslation()
  if (routes.length === 0 || models.length === 0) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Add a Responses route and channel models to view capabilities.')}
      </p>
    )
  }

  return (
    <div className='space-y-3'>
      <div>
        <h5 className='text-sm font-medium'>{t('Model Tool Capabilities')}</h5>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Effective policy and source. Evidence reflects loaded events only.'
          )}
        </p>
      </div>
      {routes.map((route) => {
        const routePath = route.incoming_path || '-'
        const routeEvents = events.filter((event) => event.route === routePath)
        return (
          <div key={routePath} className='space-y-2 rounded-md border p-2'>
            <p className='text-xs font-medium break-all'>{routePath}</p>
            <div className='grid gap-2 xl:grid-cols-2'>
              {models.map((model) => {
                const stale = !configuredModelSet.has(model)
                const override =
                  route.converter_options?.responses_tool_model_overrides?.find(
                    (candidate) =>
                      (candidate.models || []).some(
                        (candidateModel) => candidateModel.trim() === model
                      )
                  )
                return (
                  <div key={model} className='space-y-2 rounded border p-2'>
                    <div className='flex flex-wrap items-center gap-1.5'>
                      <span className='text-xs font-medium break-all'>
                        {model}
                      </span>
                      {stale ? (
                        <Badge variant='destructive'>
                          {t('Stale override')}
                        </Badge>
                      ) : null}
                    </div>
                    <div className='grid gap-1.5 sm:grid-cols-2 lg:grid-cols-3'>
                      {ADVANCED_CUSTOM_RESPONSES_TOOL_TYPES.map((toolType) => {
                        const resolution =
                          resolveAdvancedCustomResponsesToolPolicy(
                            route,
                            model,
                            toolType
                          )
                        const relatedEvents = routeEvents.filter(
                          (event) =>
                            eventMatchesModel(event, model) &&
                            normalizeAdvancedCustomResponsesToolType(
                              event.tool_type
                            ) === toolType
                        )
                        const positive = latestEvent(relatedEvents, (event) =>
                          positiveEventTypes.has(event.event_type)
                        )
                        const error = latestEvent(
                          relatedEvents,
                          (event) => !positiveEventTypes.has(event.event_type)
                        )
                        const nameRuleCount = (
                          override?.responses_tool_names || []
                        ).filter(
                          (policy) =>
                            normalizeAdvancedCustomResponsesToolType(
                              policy.tool_type || ''
                            ) === toolType
                        ).length
                        return (
                          <div
                            key={toolType}
                            className='bg-muted/25 min-w-0 rounded p-1.5 text-[11px]'
                          >
                            <p
                              className='truncate font-medium'
                              title={toolType}
                            >
                              {toolType}
                            </p>
                            <p className='break-words'>
                              {t(resolution.policy)} · {t(resolution.source)}
                            </p>
                            {nameRuleCount > 0 ? (
                              <p className='text-muted-foreground'>
                                {t('{{count}} tool-name rules', {
                                  count: nameRuleCount,
                                })}
                              </p>
                            ) : null}
                            <p className='text-muted-foreground'>
                              {t('Positive')}:{' '}
                              {positive
                                ? formatTime(positive.last_seen_at)
                                : '-'}
                            </p>
                            <p className='text-muted-foreground'>
                              {t('Error')}:{' '}
                              {error ? formatTime(error.last_seen_at) : '-'}
                              {error ? ` · ${t(error.resolution_status)}` : ''}
                            </p>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )
      })}
    </div>
  )
}
