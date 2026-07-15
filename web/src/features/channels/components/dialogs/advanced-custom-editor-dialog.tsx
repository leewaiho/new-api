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
  ArrowDown,
  ArrowDownToLine,
  ArrowRight,
  ArrowUp,
  Check,
  Info,
  Plus,
  Shuffle,
  Trash2,
  type LucideIcon,
} from 'lucide-react'
import { type ReactNode, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import {
  ADVANCED_CUSTOM_AUTH_MODE_OPTIONS,
  ADVANCED_CUSTOM_CONVERTER_OPTIONS,
  ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS,
  ADVANCED_CUSTOM_MODEL_LIST_LABEL,
  ADVANCED_CUSTOM_MODEL_LIST_PATH,
  ADVANCED_CUSTOM_RESPONSES_TOOL_POLICY_OPTIONS,
  ADVANCED_CUSTOM_RESPONSES_TOOLS_MODE_OPTIONS,
  ADVANCED_CUSTOM_TEMPLATE_OPTIONS,
  type AdvancedCustomAuthMode,
  buildAdvancedCustomAuth,
  createAdvancedCustomConfig,
  createAdvancedCustomRoute,
  createDualEndpointConfig,
  getAdvancedCustomAuthMode,
  getAdvancedCustomConverterDefaults,
  getAdvancedCustomConverterOptions,
  getAdvancedCustomIncomingPathLabel,
  getAdvancedCustomModelRuleKind,
  getAdvancedCustomRegexModelPattern,
  ADVANCED_CUSTOM_RESPONSES_DROP_FIELDS,
  type AdvancedCustomResponsesDropField,
  type AdvancedCustomResponsesToolType,
  getAdvancedCustomTemplateConfig,
  getAdvancedCustomUpstreamPathPlaceholder,
  getDefaultAdvancedCustomIncomingPath,
  isAdvancedCustomIncomingPathAllowed,
  mergeAdvancedCustomRouteCompatibilityConfig,
  normalizeAdvancedCustomConfig,
  parseAdvancedCustomRouteModels,
  parseAdvancedCustomConfig,
  resolveAdvancedCustomResponsesToolPolicy,
  responsesToolsFromMode,
  stringifyAdvancedCustomConfig,
  validateAdvancedCustomConfig,
} from '../../lib/advanced-custom'
import type {
  AdvancedCustomAuthType,
  AdvancedCustomConfig,
  AdvancedCustomConverter,
  AdvancedCustomResponsesToolConflictPolicy,
  AdvancedCustomResponsesToolModelOverride,
  AdvancedCustomResponsesToolNamePolicy,
  AdvancedCustomResponsesToolPolicy,
  AdvancedCustomResponsesToolsMode,
  AdvancedCustomRoute,
} from '../../types'
import { ToolCompatibilityEvents } from '../tool-compatibility-events'

type AdvancedCustomEditorDialogProps = {
  open: boolean
  value: string
  channelId?: number
  channelModels?: string[]
  onOpenChange: (open: boolean) => void
  onSave: (value: string) => void
}

type AdvancedCustomEditMode = 'visual' | 'json'

const longSelectContentClass = 'w-[360px] max-w-[calc(100vw-2rem)]'
const longSelectItemClass =
  'items-start py-2 [&_[data-slot=select-item-text]]:min-w-0 [&_[data-slot=select-item-text]]:shrink [&_[data-slot=select-item-text]]:whitespace-normal'
const routeEditorGridClassName =
  'lg:grid-cols-[6rem_minmax(0,1fr)_minmax(0,1.25fr)_minmax(0,1fr)_minmax(0,0.85fr)_7rem]'
const responseToolPolicyGridClassName =
  'lg:grid-cols-[7rem_repeat(6,minmax(0,1fr))]'
const upstreamPathDescriptionKey =
  'Use a path to append it to the channel Base URL, or enter a full URL to override the Base URL for this route.'
const catchAllOrderErrorMessage =
  'Catch-all route must be last for the same incoming path'
const emptyAdvancedRoutes: AdvancedCustomRoute[] = []

type AdvancedCustomRouteRow = {
  route: AdvancedCustomRoute
  routeKey: string
  index: number
}

type AdvancedCustomRouteGroup = {
  incomingPath: string
  routeRows: AdvancedCustomRouteRow[]
}

function getOptionLabel(
  options: ReadonlyArray<{ value: string; label: string }>,
  value: string
) {
  return options.find((option) => option.value === value)?.label || value
}

function getRouteIncomingPath(route: AdvancedCustomRoute): string {
  return (route.incoming_path || '').trim()
}

function isCatchAllRoute(route: AdvancedCustomRoute): boolean {
  return !route.models || route.models.length === 0
}

function buildRouteGroups(
  routeRows: AdvancedCustomRouteRow[]
): AdvancedCustomRouteGroup[] {
  const groups: AdvancedCustomRouteGroup[] = []
  const groupByPath = new Map<string, AdvancedCustomRouteGroup>()

  for (const routeRow of routeRows) {
    const incomingPath = getRouteIncomingPath(routeRow.route)
    let group = groupByPath.get(incomingPath)
    if (!group) {
      group = { incomingPath, routeRows: [] }
      groupByPath.set(incomingPath, group)
      groups.push(group)
    }
    group.routeRows.push(routeRow)
  }

  return groups
}

export function AdvancedCustomEditorDialog({
  open,
  value,
  channelId,
  channelModels,
  onOpenChange,
  onSave,
}: AdvancedCustomEditorDialogProps) {
  const { t } = useTranslation()
  const routeKeyCounterRef = useRef(0)
  const [config, setConfig] = useState<AdvancedCustomConfig>(
    () => parseAdvancedCustomConfig(value) || createAdvancedCustomConfig()
  )
  const [routeKeys, setRouteKeys] = useState<string[]>(() => {
    const initialConfig =
      parseAdvancedCustomConfig(value) || createAdvancedCustomConfig()
    const normalized = normalizeAdvancedCustomConfig(initialConfig)
    return (normalized.advanced_routes || []).map(
      (_, routeIndex) => `advanced-custom-route-initial-${routeIndex}`
    )
  })
  const [editMode, setEditMode] = useState<AdvancedCustomEditMode>('visual')
  const [jsonText, setJsonText] = useState(() =>
    stringifyAdvancedCustomConfig(
      parseAdvancedCustomConfig(value) || createAdvancedCustomConfig()
    )
  )
  const [jsonError, setJsonError] = useState('')
  const [templateKey, setTemplateKey] = useState(
    ADVANCED_CUSTOM_TEMPLATE_OPTIONS[0]?.value || ''
  )
  const templateLabel = useMemo(
    () => getOptionLabel(ADVANCED_CUSTOM_TEMPLATE_OPTIONS, templateKey),
    [templateKey]
  )

  const normalizedConfig = useMemo(
    () => normalizeAdvancedCustomConfig(config),
    [config]
  )
  const routes = normalizedConfig.advanced_routes || emptyAdvancedRoutes
  const routeRows = useMemo(
    () =>
      routes.map((route, index) => ({
        route,
        index,
        routeKey:
          routeKeys.at(index) ||
          route.incoming_path ||
          route.upstream_path ||
          route.converter ||
          'advanced-custom-route',
      })),
    [routeKeys, routes]
  )
  const routeGroups = useMemo(() => buildRouteGroups(routeRows), [routeRows])
  const usedIncomingPaths = useMemo(
    () => new Set(routeGroups.map((routeGroup) => routeGroup.incomingPath)),
    [routeGroups]
  )
  const availableIncomingPathOptions = useMemo(
    () =>
      ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS.filter(
        (option) => !usedIncomingPaths.has(option.value)
      ),
    [usedIncomingPaths]
  )
  const validationError = useMemo(
    () => validateAdvancedCustomConfig(normalizedConfig),
    [normalizedConfig]
  )
  const canFixCatchAllOrder =
    validationError?.message === catchAllOrderErrorMessage

  const createRouteKey = () => {
    routeKeyCounterRef.current += 1
    return `advanced-custom-route-${routeKeyCounterRef.current}`
  }

  const createRouteKeys = (count: number) =>
    Array.from({ length: count }, () => createRouteKey())

  const updateRoute = (index: number, patch: Partial<AdvancedCustomRoute>) => {
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      const nextRoutes = [...(next.advanced_routes || [])]
      nextRoutes[index] = { ...nextRoutes[index], ...patch }
      return { ...next, advanced_routes: nextRoutes }
    })
  }

  const replaceRoutes = (
    nextRoutes: AdvancedCustomRoute[],
    nextRouteKeys = routeRows.map((routeRow) => routeRow.routeKey)
  ) => {
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return { ...next, advanced_routes: nextRoutes }
    })
    setRouteKeys(nextRouteKeys)
  }

  const addRoute = (incomingPath: string | null) => {
    if (!incomingPath || usedIncomingPaths.has(incomingPath)) return
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return {
        ...next,
        advanced_routes: [
          ...(next.advanced_routes || []),
          {
            ...createAdvancedCustomRoute(),
            incoming_path: incomingPath,
            upstream_path: incomingPath,
          },
        ],
      }
    })
    setRouteKeys((current) => [...current, createRouteKey()])
  }

  const addRouteForIncomingPath = (incomingPath: string) => {
    const resolvedIncomingPath = incomingPath || '/v1/chat/completions'
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return {
        ...next,
        advanced_routes: [
          ...(next.advanced_routes || []),
          {
            ...createAdvancedCustomRoute(),
            incoming_path: resolvedIncomingPath,
            upstream_path: resolvedIncomingPath,
          },
        ],
      }
    })
    setRouteKeys((current) => [...current, createRouteKey()])
  }

  const removeRoute = (index: number) => {
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return {
        ...next,
        advanced_routes: (next.advanced_routes || []).filter(
          (_, routeIndex) => routeIndex !== index
        ),
      }
    })
    setRouteKeys((current) =>
      current.filter((_, routeIndex) => routeIndex !== index)
    )
  }

  const updateGroupIncomingPath = (
    group: AdvancedCustomRouteGroup,
    nextIncomingPath: string | null
  ) => {
    const resolvedIncomingPath = nextIncomingPath || '/v1/chat/completions'
    const groupRouteIndexes = new Set(
      group.routeRows.map((routeRow) => routeRow.index)
    )
    const nextRoutes = routes.map((route, routeIndex) => {
      if (!groupRouteIndexes.has(routeIndex)) return route
      if (resolvedIncomingPath === ADVANCED_CUSTOM_MODEL_LIST_PATH) {
        return {
          ...route,
          incoming_path: resolvedIncomingPath,
          upstream_path: ADVANCED_CUSTOM_MODEL_LIST_PATH,
          converter: 'none' as const,
          models: [],
        }
      }
      const converter = route.converter || 'none'
      return {
        ...route,
        incoming_path: resolvedIncomingPath,
        converter: isAdvancedCustomIncomingPathAllowed(
          resolvedIncomingPath,
          converter
        )
          ? converter
          : 'none',
      }
    })
    replaceRoutes(nextRoutes)
  }

  const swapRoutes = (fromIndex: number, toIndex: number) => {
    if (fromIndex === toIndex) return
    const nextRoutes = [...routes]
    const nextRouteKeys = routeRows.map((routeRow) => routeRow.routeKey)
    const fromRoute = nextRoutes[fromIndex]
    nextRoutes[fromIndex] = nextRoutes[toIndex]
    nextRoutes[toIndex] = fromRoute
    const fromRouteKey = nextRouteKeys[fromIndex]
    nextRouteKeys[fromIndex] = nextRouteKeys[toIndex]
    nextRouteKeys[toIndex] = fromRouteKey
    replaceRoutes(nextRoutes, nextRouteKeys)
  }

  const moveRouteWithinGroup = (index: number, direction: -1 | 1) => {
    const incomingPath = getRouteIncomingPath(routes[index])
    const samePathIndexes = routes
      .map((route, routeIndex) => ({ route, routeIndex }))
      .filter(({ route }) => getRouteIncomingPath(route) === incomingPath)
      .map(({ routeIndex }) => routeIndex)
    const position = samePathIndexes.indexOf(index)
    const nextIndex = samePathIndexes.at(position + direction)
    if (nextIndex === undefined) return
    swapRoutes(index, nextIndex)
  }

  const moveRouteToGroupEnd = (index: number) => {
    const incomingPath = getRouteIncomingPath(routes[index])
    let lastSamePathIndex = -1
    for (let routeIndex = routes.length - 1; routeIndex >= 0; routeIndex -= 1) {
      if (getRouteIncomingPath(routes[routeIndex]) === incomingPath) {
        lastSamePathIndex = routeIndex
        break
      }
    }
    if (lastSamePathIndex < 0 || index === lastSamePathIndex) return

    const nextRoutes = [...routes]
    const nextRouteKeys = routeRows.map((routeRow) => routeRow.routeKey)
    const [route] = nextRoutes.splice(index, 1)
    const [routeKey] = nextRouteKeys.splice(index, 1)
    nextRoutes.splice(lastSamePathIndex, 0, route)
    nextRouteKeys.splice(lastSamePathIndex, 0, routeKey)
    replaceRoutes(nextRoutes, nextRouteKeys)
  }

  const fixCatchAllOrder = () => {
    const routeRowsByPath = new Map<string, AdvancedCustomRouteRow[]>()
    for (const routeRow of routeRows) {
      const incomingPath = getRouteIncomingPath(routeRow.route)
      routeRowsByPath.set(incomingPath, [
        ...(routeRowsByPath.get(incomingPath) || []),
        routeRow,
      ])
    }

    const orderedRowsByPath = new Map<string, AdvancedCustomRouteRow[]>()
    for (const [incomingPath, rows] of routeRowsByPath) {
      orderedRowsByPath.set(incomingPath, [
        ...rows.filter((routeRow) => !isCatchAllRoute(routeRow.route)),
        ...rows.filter((routeRow) => isCatchAllRoute(routeRow.route)),
      ])
    }

    const nextRows = routeRows.map((routeRow) => {
      const incomingPath = getRouteIncomingPath(routeRow.route)
      const orderedRows = orderedRowsByPath.get(incomingPath)
      return orderedRows?.shift() || routeRow
    })
    replaceRoutes(
      nextRows.map((routeRow) => routeRow.route),
      nextRows.map((routeRow) => routeRow.routeKey)
    )
  }

  const parseJsonEditorConfig = (): AdvancedCustomConfig | null => {
    const parsed = parseAdvancedCustomConfig(jsonText)
    if (!parsed) {
      setJsonError(t('Invalid JSON'))
      return null
    }

    const error = validateAdvancedCustomConfig(parsed)
    if (error) {
      setJsonError(t(error.message))
      return null
    }

    setJsonError('')
    return parsed
  }

  const switchToVisualMode = () => {
    const parsed = parseJsonEditorConfig()
    if (!parsed) return
    const normalized = normalizeAdvancedCustomConfig(parsed)
    setConfig(normalized)
    setRouteKeys(createRouteKeys(normalized.advanced_routes?.length || 0))
    setEditMode('visual')
  }

  const switchToJsonMode = () => {
    setJsonText(stringifyAdvancedCustomConfig(normalizedConfig))
    setJsonError('')
    setEditMode('json')
  }

  const handleJsonChange = (nextValue: string) => {
    setJsonText(nextValue)
    if (jsonError) setJsonError('')
  }

  const applyTemplate = (mode: 'fill' | 'append') => {
    const templateConfig = getAdvancedCustomTemplateConfig(templateKey)
    let nextConfig = templateConfig

    if (mode === 'append') {
      const baseConfig =
        editMode === 'json' ? parseJsonEditorConfig() : normalizedConfig
      if (!baseConfig) return
      const base = normalizeAdvancedCustomConfig(baseConfig)
      const template = normalizeAdvancedCustomConfig(templateConfig)
      nextConfig = {
        ...base,
        advanced_routes: [
          ...(base.advanced_routes || []),
          ...(template.advanced_routes || []),
        ],
      }
    }

    const normalized = normalizeAdvancedCustomConfig(nextConfig)
    setConfig(normalized)
    setRouteKeys(createRouteKeys(normalized.advanced_routes?.length || 0))
    setJsonText(stringifyAdvancedCustomConfig(normalized))
    setJsonError('')
  }

  const saveConfig = () => {
    if (editMode === 'json') {
      const parsed = parseJsonEditorConfig()
      if (!parsed) {
        toast.error(t('Please fix JSON errors before saving'))
        return
      }
      onSave(stringifyAdvancedCustomConfig(parsed))
      onOpenChange(false)
      return
    }

    if (validationError) {
      toast.error(t(validationError.message))
      return
    }
    onSave(stringifyAdvancedCustomConfig(normalizedConfig))
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Advanced Custom Routes')}
      description={t('Advanced Custom')}
      contentClassName='flex max-h-[90vh] flex-col gap-0 p-0 sm:max-w-5xl'
      headerClassName='border-b px-6 py-4'
      footerClassName='border-t px-6 py-4'
      contentHeight='70vh'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={saveConfig}>
            <Check data-icon='inline-start' />
            {t('Save changes')}
          </Button>
        </>
      }
    >
      <div className='bg-muted/30 border-b px-4 py-3'>
        <div className='flex flex-wrap items-center gap-2'>
          <span className='text-muted-foreground text-xs font-medium'>
            {t('Mode')}
          </span>
          <Button
            type='button'
            variant={editMode === 'visual' ? 'default' : 'outline'}
            size='sm'
            onClick={switchToVisualMode}
          >
            {t('Visual')}
          </Button>
          <Button
            type='button'
            variant={editMode === 'json' ? 'default' : 'outline'}
            size='sm'
            onClick={switchToJsonMode}
          >
            {t('JSON Text')}
          </Button>

          <div className='bg-border mx-1 h-5 w-px' />

          <span className='text-muted-foreground text-xs font-medium'>
            {t('Template')}
          </span>
          <Select
            value={templateKey}
            onValueChange={(nextValue) =>
              setTemplateKey(
                nextValue || ADVANCED_CUSTOM_TEMPLATE_OPTIONS[0]?.value || ''
              )
            }
          >
            <SelectTrigger className='h-8 max-w-full min-w-[260px] flex-1 sm:w-[320px]'>
              <SelectValue className='min-w-0 truncate'>
                {t(templateLabel)}
              </SelectValue>
            </SelectTrigger>
            <SelectContent
              alignItemWithTrigger={false}
              className={longSelectContentClass}
            >
              <SelectGroup>
                {ADVANCED_CUSTOM_TEMPLATE_OPTIONS.map((option) => (
                  <SelectItem
                    key={option.value}
                    value={option.value}
                    className={longSelectItemClass}
                  >
                    <span className='min-w-0 leading-snug break-words whitespace-normal'>
                      {t(option.label)}
                    </span>
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => applyTemplate('fill')}
          >
            {t('Fill Template')}
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={() => applyTemplate('append')}
          >
            {t('Append Template')}
          </Button>

          <div className='bg-border mx-1 h-5 w-px' />

          <DualEndpointQuickSetup
            onApply={(generatedConfig) => {
              const normalized = normalizeAdvancedCustomConfig(generatedConfig)
              setConfig(normalized)
              setRouteKeys(
                createRouteKeys(normalized.advanced_routes?.length || 0)
              )
              toast.success(t('Quick setup applied'))
            }}
          />
        </div>
      </div>

      {editMode === 'visual' ? (
        <div className='flex flex-col gap-4 p-4 lg:gap-3'>
          <div className='flex justify-end border-y py-4 lg:py-2'>
            <Select
              items={availableIncomingPathOptions}
              value={null}
              onValueChange={(incomingPath) => {
                if (typeof incomingPath === 'string') {
                  addRoute(incomingPath)
                }
              }}
            >
              <SelectTrigger
                size='sm'
                disabled={availableIncomingPathOptions.length === 0}
              >
                <Plus data-icon='inline-start' />
                <SelectValue placeholder={t('Add route')} />
              </SelectTrigger>
              <SelectContent
                align='end'
                alignItemWithTrigger={false}
                className={longSelectContentClass}
              >
                <SelectGroup>
                  {availableIncomingPathOptions.map((option) => (
                    <SelectItem
                      key={option.value}
                      value={option.value}
                      className={longSelectItemClass}
                    >
                      <div className='flex min-w-0 flex-col gap-1 leading-snug whitespace-normal'>
                        <span>{option.label}</span>
                        <span className='text-muted-foreground font-mono text-xs break-all'>
                          {option.value}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          {validationError ? (
            <Alert variant='destructive'>
              <AlertDescription className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                <span>
                  {validationError.routeIndex !== undefined
                    ? `${t('Route')} ${validationError.routeIndex + 1}: `
                    : ''}
                  {t(validationError.message)}
                </span>
                {canFixCatchAllOrder ? (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={fixCatchAllOrder}
                  >
                    {t('Fix order')}
                  </Button>
                ) : null}
              </AlertDescription>
            </Alert>
          ) : null}

          <p className='text-muted-foreground bg-muted/30 hidden rounded-md border px-3 py-2 text-xs leading-relaxed lg:block'>
            {t(upstreamPathDescriptionKey)}
          </p>

          <div className='flex flex-col gap-4'>
            {routeGroups.map((routeGroup) => (
              <RouteGroupEditor
                key={routeGroup.incomingPath || 'advanced-custom-empty-path'}
                group={routeGroup}
                usedIncomingPaths={usedIncomingPaths}
                validationError={validationError}
                onAddRoute={() =>
                  addRouteForIncomingPath(routeGroup.incomingPath)
                }
                onIncomingPathChange={(nextIncomingPath) =>
                  updateGroupIncomingPath(routeGroup, nextIncomingPath)
                }
                onMoveRoute={(index, direction) =>
                  moveRouteWithinGroup(index, direction)
                }
                onMoveRouteToEnd={moveRouteToGroupEnd}
                onRemoveRoute={removeRoute}
                onRouteChange={updateRoute}
                channelModels={channelModels || []}
              />
            ))}
          </div>
        </div>
      ) : (
        <div className='p-4'>
          <div className='mb-2 flex items-center gap-2'>
            <span className='text-muted-foreground text-xs'>
              {t('Advanced text editing')}
            </span>
          </div>
          <JsonCodeEditor
            value={jsonText}
            onChange={handleJsonChange}
            placeholder={stringifyAdvancedCustomConfig(
              getAdvancedCustomTemplateConfig(templateKey)
            )}
            heightClassName='h-[420px] min-h-[420px] max-h-[420px]'
            aria-invalid={Boolean(jsonError)}
            ariaLabel={t('Advanced text editing')}
          />
          <p className='text-muted-foreground mt-2 text-xs'>
            {t('Edit JSON text directly. Format will be validated on save.')}
          </p>
          {jsonError ? (
            <p className='text-destructive mt-1 text-xs'>{jsonError}</p>
          ) : null}
        </div>
      )}
      {editMode === 'visual' ? (
        <ToolCompatibilityEvents
          channelId={channelId}
          channelModels={channelModels}
          routes={normalizedConfig.advanced_routes || []}
          onRouteConfigChange={(routeConfig) =>
            setConfig((current) =>
              mergeAdvancedCustomRouteCompatibilityConfig(current, routeConfig)
            )
          }
        />
      ) : null}
    </Dialog>
  )
}

function RouteGroupEditor({
  group,
  usedIncomingPaths,
  validationError,
  onAddRoute,
  onIncomingPathChange,
  onMoveRoute,
  onMoveRouteToEnd,
  onRemoveRoute,
  onRouteChange,
  channelModels,
}: {
  group: AdvancedCustomRouteGroup
  usedIncomingPaths: ReadonlySet<string>
  validationError: ReturnType<typeof validateAdvancedCustomConfig>
  onAddRoute: () => void
  onIncomingPathChange: (incomingPath: string | null) => void
  onMoveRoute: (index: number, direction: -1 | 1) => void
  onMoveRouteToEnd: (index: number) => void
  onRemoveRoute: (index: number) => void
  onRouteChange: (index: number, patch: Partial<AdvancedCustomRoute>) => void
  channelModels: string[]
}) {
  const { t } = useTranslation()
  const incomingPath = group.incomingPath || '/v1/chat/completions'
  const isModelListGroup = incomingPath === ADVANCED_CUSTOM_MODEL_LIST_PATH
  const incomingPathLabel = getAdvancedCustomIncomingPathLabel(incomingPath)
  const catchAllRoute = group.routeRows.find((routeRow) =>
    isCatchAllRoute(routeRow.route)
  )
  const catchAllRoutePosition = catchAllRoute
    ? group.routeRows.findIndex(
        (routeRow) => routeRow.index === catchAllRoute.index
      )
    : -1
  const hasCatchAll = catchAllRoute !== undefined
  const catchAllIsLast =
    !hasCatchAll || catchAllRoutePosition === group.routeRows.length - 1
  const groupHasError =
    validationError?.routeIndex !== undefined &&
    group.routeRows.some(
      (routeRow) => routeRow.index === validationError.routeIndex
    )

  return (
    <section
      className={cn(
        'border-border overflow-hidden rounded-md border',
        groupHasError && 'border-destructive/60'
      )}
    >
      <div className='bg-muted/20 flex flex-col gap-3 p-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex min-w-0 flex-1 flex-col gap-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-sm font-medium'>{t('Route group')}</span>
            <Badge variant='secondary'>
              {group.routeRows.length} {t('Routes')}
            </Badge>
            {isModelListGroup ? (
              <Badge variant='outline'>
                {ADVANCED_CUSTOM_MODEL_LIST_LABEL}
              </Badge>
            ) : (
              <Badge variant={hasCatchAll ? 'outline' : 'secondary'}>
                {hasCatchAll ? t('Fallback route') : t('Model-scoped only')}
              </Badge>
            )}
            {!isModelListGroup && !catchAllIsLast ? (
              <Badge variant='destructive'>{t('Fallback must be last')}</Badge>
            ) : null}
          </div>
          <Select
            items={ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS}
            value={incomingPath}
            onValueChange={onIncomingPathChange}
          >
            <SelectTrigger className='h-9 max-w-full lg:max-w-[420px]'>
              <SelectValue className='min-w-0 truncate'>
                {incomingPathLabel}
              </SelectValue>
            </SelectTrigger>
            <SelectContent
              alignItemWithTrigger={false}
              className={longSelectContentClass}
            >
              <SelectGroup>
                {ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS.map((option) => (
                  <SelectItem
                    key={option.value}
                    value={option.value}
                    disabled={
                      (option.value !== incomingPath &&
                        usedIncomingPaths.has(option.value)) ||
                      (option.value === ADVANCED_CUSTOM_MODEL_LIST_PATH &&
                        group.routeRows.length > 1)
                    }
                    className={longSelectItemClass}
                  >
                    <div className='flex min-w-0 flex-col gap-1 leading-snug whitespace-normal'>
                      <span>{option.label}</span>
                      <span className='text-muted-foreground font-mono text-xs break-all'>
                        {option.value}
                      </span>
                    </div>
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>

        {!isModelListGroup ? (
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={onAddRoute}
          >
            <Plus data-icon='inline-start' />
            {t('Add split')}
          </Button>
        ) : null}
      </div>

      <div className='border-t px-3 py-2'>
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {isModelListGroup
            ? t(
                'This route discovers upstream OpenAI models and cannot be split or matched by client model rules.'
              )
            : t(
                'Routes with the same incoming path are split by client model rules. Unmatched requests use the final fallback.'
              )}
        </p>
        {groupHasError && validationError ? (
          <p className='text-destructive mt-1 text-xs'>
            {validationError.routeIndex !== undefined
              ? `${t('Route')} ${validationError.routeIndex + 1}: `
              : ''}
            {t(validationError.message)}
          </p>
        ) : null}
      </div>

      <div
        className={cn(
          'text-muted-foreground hidden items-center gap-2 border-t bg-muted/10 px-3 py-2 text-xs font-medium lg:grid',
          routeEditorGridClassName
        )}
      >
        <span>{t('Route')}</span>
        <span className='inline-flex items-center gap-1'>
          {t('Client model')}
          <ModelRuleHelpPopover />
        </span>
        <span>{t('Upstream path')}</span>
        <span>{t('Converter')}</span>
        <span>{t('Auth')}</span>
        <span className='text-right'>{t('Actions')}</span>
      </div>

      <div className='divide-y'>
        {group.routeRows.map((routeRow, position) => {
          const canMoveUp = position > 0
          const canMoveDown = position < group.routeRows.length - 1
          const catchAllOutOfOrder =
            isCatchAllRoute(routeRow.route) && canMoveDown
          const routeErrorMessage =
            validationError?.routeIndex === routeRow.index
              ? validationError.message
              : undefined

          return (
            <RouteEditor
              key={routeRow.routeKey}
              route={routeRow.route}
              index={routeRow.index}
              errorMessage={routeErrorMessage}
              canMoveUp={canMoveUp}
              canMoveDown={canMoveDown}
              catchAllOutOfOrder={catchAllOutOfOrder}
              onChange={(patch) => onRouteChange(routeRow.index, patch)}
              onMoveDown={() => onMoveRoute(routeRow.index, 1)}
              onMoveUp={() => onMoveRoute(routeRow.index, -1)}
              onMoveCatchAllToEnd={() => onMoveRouteToEnd(routeRow.index)}
              onRemove={() => onRemoveRoute(routeRow.index)}
              channelModels={channelModels}
            />
          )
        })}
      </div>
    </section>
  )
}

const responseToolPolicyFields: Array<{
  key: Exclude<AdvancedCustomResponsesToolType, 'function'>
  label: string
  allowFlatten: boolean
}> = [
  { key: 'namespace', label: 'Namespace', allowFlatten: true },
  { key: 'custom', label: 'Custom', allowFlatten: false },
  { key: 'web_search', label: 'Web search', allowFlatten: false },
  { key: 'tool_search', label: 'Tool search', allowFlatten: false },
  { key: 'image_generation', label: 'Image generation', allowFlatten: false },
  { key: 'unknown', label: 'Unknown / other', allowFlatten: false },
]

function RouteEditor({
  route,
  index,
  errorMessage,
  canMoveUp,
  canMoveDown,
  catchAllOutOfOrder,
  onChange,
  onMoveUp,
  onMoveDown,
  onMoveCatchAllToEnd,
  onRemove,
  channelModels,
}: {
  route: AdvancedCustomRoute
  index: number
  errorMessage?: string
  canMoveUp: boolean
  canMoveDown: boolean
  catchAllOutOfOrder: boolean
  onChange: (patch: Partial<AdvancedCustomRoute>) => void
  onMoveUp: () => void
  onMoveDown: () => void
  onMoveCatchAllToEnd: () => void
  onRemove: () => void
  channelModels: string[]
}) {
  const { t } = useTranslation()
  const converter = route.converter || 'none'
  const authMode = getAdvancedCustomAuthMode(route)
  const incomingPath =
    route.incoming_path || getDefaultAdvancedCustomIncomingPath(converter)
  const isModelListRoute = incomingPath === ADVANCED_CUSTOM_MODEL_LIST_PATH
  const converterOptions = useMemo(
    () => getAdvancedCustomConverterOptions(incomingPath),
    [incomingPath]
  )
  const converterLabel = getOptionLabel(
    ADVANCED_CUSTOM_CONVERTER_OPTIONS,
    converter
  )
  const converterTriggerLabel =
    ADVANCED_CUSTOM_CONVERTER_OPTIONS.find(
      (option) => option.value === converter
    )?.triggerLabel || converterLabel
  const authLabel = getOptionLabel(ADVANCED_CUSTOM_AUTH_MODE_OPTIONS, authMode)
  const explicitResponsesToolsMode =
    route.converter_options?.responses_tools_mode
  const hasCustomPerToolPolicies =
    !explicitResponsesToolsMode &&
    Boolean(route.converter_options?.responses_tools)
  const responsesToolsMode: AdvancedCustomResponsesToolsMode =
    explicitResponsesToolsMode ||
    (converter === 'none' ? 'preserve' : 'compat_flatten')
  const responsesToolsModeLabel = hasCustomPerToolPolicies
    ? 'Custom per-tool policies'
    : getOptionLabel(
        ADVANCED_CUSTOM_RESPONSES_TOOLS_MODE_OPTIONS,
        responsesToolsMode
      )
  const responseToolPolicies = useMemo(
    () => ({
      namespace: resolveAdvancedCustomResponsesToolPolicy(
        route,
        '',
        'namespace'
      ).policy,
      custom: resolveAdvancedCustomResponsesToolPolicy(route, '', 'custom')
        .policy,
      web_search: resolveAdvancedCustomResponsesToolPolicy(
        route,
        '',
        'web_search'
      ).policy,
      tool_search: resolveAdvancedCustomResponsesToolPolicy(
        route,
        '',
        'tool_search'
      ).policy,
      image_generation: resolveAdvancedCustomResponsesToolPolicy(
        route,
        '',
        'image_generation'
      ).policy,
      unknown: resolveAdvancedCustomResponsesToolPolicy(route, '', 'unknown')
        .policy,
    }),
    [route]
  )
  const hasResponsesToolOptions =
    Boolean(route.converter_options?.responses_tools_mode) ||
    Boolean(route.converter_options?.responses_tools)
  const isNativeConverter = converter === 'none'
  const isResponsesRoute =
    incomingPath === '/v1/responses' || incomingPath === '/v1/responses/compact'
  const supportsResponsesToolHandling =
    isResponsesRoute &&
    (isNativeConverter ||
      converter === 'openai_responses_to_openai_chat_completions')
  const ConverterVisualIcon = isNativeConverter ? ArrowRight : Shuffle
  const modelsInputValue = route.models?.join(', ') || ''
  const parsedRouteModels = parseAdvancedCustomRouteModels(modelsInputValue)
  const isFallback = !isModelListRoute && parsedRouteModels.length === 0

  const setConverter = (nextConverter: AdvancedCustomConverter) => {
    let nextIncomingPath = incomingPath
    if (!isAdvancedCustomIncomingPathAllowed(nextIncomingPath, nextConverter)) {
      nextIncomingPath = getDefaultAdvancedCustomIncomingPath(nextConverter)
    }
    const defaults = getAdvancedCustomConverterDefaults(
      nextConverter,
      nextIncomingPath
    )
    const nextSupportsResponsesToolHandling =
      (nextIncomingPath === '/v1/responses' ||
        nextIncomingPath === '/v1/responses/compact') &&
      (nextConverter === 'none' ||
        nextConverter === 'openai_responses_to_openai_chat_completions')
    onChange({
      converter: nextConverter,
      incoming_path: nextIncomingPath,
      upstream_path: defaults.upstream_path,
      auth: defaults.auth,
      converter_options:
        nextSupportsResponsesToolHandling
          ? hasResponsesToolOptions
            ? route.converter_options
            : {
                responses_tools_mode:
                  nextConverter === 'none' ? 'preserve' : 'compat_flatten',
                responses_tools: responsesToolsFromMode(
                  nextConverter === 'none' ? 'preserve' : 'compat_flatten'
                ),
              }
          : undefined,
    })
  }

  const setAuthMode = (mode: AdvancedCustomAuthMode) => {
    onChange({ auth: buildAdvancedCustomAuth(mode, route.auth) })
  }

  const setModelsInput = (value: string) => {
    onChange({
      models: value === '' ? [] : value.split(','),
    })
  }

  const normalizeModelsInput = (value: string) => {
    onChange({ models: parseAdvancedCustomRouteModels(value) })
  }

  const setResponsesToolsMode = (mode: AdvancedCustomResponsesToolsMode) => {
    onChange({
      converter_options: {
        ...route.converter_options,
        responses_tools_mode: mode,
        responses_tools: responsesToolsFromMode(mode),
      },
    })
  }

  const setResponseToolPolicy = (
    key: Exclude<AdvancedCustomResponsesToolType, 'function'>,
    policy: AdvancedCustomResponsesToolPolicy
  ) => {
    onChange({
      converter_options: {
        ...route.converter_options,
        responses_tools: {
          ...route.converter_options?.responses_tools,
          [key]: policy,
        },
      },
    })
  }

  const responseToolConflictPolicy: AdvancedCustomResponsesToolConflictPolicy =
    route.converter_options?.responses_tool_conflict_policy || 'deduplicate'
  const modelToolOverrides = useMemo(
    () => route.converter_options?.responses_tool_model_overrides || [],
    [route.converter_options?.responses_tool_model_overrides]
  )
  const configuredModels = useMemo(
    () => new Set(channelModels.map((model) => model.trim()).filter(Boolean)),
    [channelModels]
  )
  const staleOverrideModels = useMemo(
    () => [
      ...new Set(
        modelToolOverrides
          .flatMap((override) => override.models || [])
          .map((model) => model.trim())
          .filter((model) => model && !configuredModels.has(model))
      ),
    ],
    [configuredModels, modelToolOverrides]
  )

  const patchConverterOptions = (
    patch: Partial<NonNullable<AdvancedCustomRoute['converter_options']>>
  ) => {
    onChange({
      converter_options: { ...route.converter_options, ...patch },
    })
  }
  const setModelToolOverride = (
    index: number,
    patch: Partial<AdvancedCustomResponsesToolModelOverride>
  ) => {
    patchConverterOptions({
      responses_tool_model_overrides: modelToolOverrides.map(
        (override, currentIndex) =>
          currentIndex === index ? { ...override, ...patch } : override
      ),
    })
  }
  const setModelToolOverridePolicy = (
    index: number,
    key: Exclude<AdvancedCustomResponsesToolType, 'function'>,
    policy: AdvancedCustomResponsesToolPolicy | 'inherit'
  ) => {
    const override = modelToolOverrides[index]
    const nextPolicies = { ...override.responses_tools }
    if (policy === 'inherit') {
      delete nextPolicies[key]
    } else {
      nextPolicies[key] = policy
    }
    setModelToolOverride(index, {
      responses_tools: nextPolicies,
    })
  }
  const removeModelToolOverride = (index: number) =>
    patchConverterOptions({
      responses_tool_model_overrides: modelToolOverrides.filter(
        (_, currentIndex) => currentIndex !== index
      ),
    })
  const addModelToolOverride = () =>
    patchConverterOptions({
      responses_tool_model_overrides: [
        ...modelToolOverrides,
        { models: [], responses_tools: {}, responses_tool_names: [] },
      ],
    })
  const setNamePolicy = (
    overrideIndex: number,
    policyIndex: number,
    patch: Partial<AdvancedCustomResponsesToolNamePolicy>
  ) => {
    const override = modelToolOverrides[overrideIndex]
    const policies = override.responses_tool_names || []
    setModelToolOverride(overrideIndex, {
      responses_tool_names: policies.map((policy, currentIndex) =>
        currentIndex === policyIndex ? { ...policy, ...patch } : policy
      ),
    })
  }
  const addNamePolicy = (overrideIndex: number) => {
    const override = modelToolOverrides[overrideIndex]
    setModelToolOverride(overrideIndex, {
      responses_tool_names: [
        ...(override.responses_tool_names || []),
        { tool_type: 'web_search', tool_name: '', policy: 'preserve' },
      ],
    })
  }
  const removeNamePolicy = (overrideIndex: number, policyIndex: number) => {
    const override = modelToolOverrides[overrideIndex]
    setModelToolOverride(overrideIndex, {
      responses_tool_names: (override.responses_tool_names || []).filter(
        (_, currentIndex) => currentIndex !== policyIndex
      ),
    })
  }

  const responsesDropFields = useMemo(
    () =>
      (route.converter_options?.responses_drop_fields || [])
        .map((f) => f.trim())
        .filter((f): f is AdvancedCustomResponsesDropField =>
          ADVANCED_CUSTOM_RESPONSES_DROP_FIELDS.some(
            (option) => option.value === f
          )
        ),
    [route.converter_options?.responses_drop_fields]
  )

  const setResponsesDropField = (
    field: AdvancedCustomResponsesDropField,
    enabled: boolean
  ) => {
    const current = (route.converter_options?.responses_drop_fields || []).map(
      (f) => f.trim()
    )
    const next = enabled
      ? [...new Set([...current, field])]
      : current.filter((f) => f !== field)
    onChange({
      converter_options: {
        ...route.converter_options,
        responses_drop_fields: next,
      },
    })
  }

  const updateAuth = (
    field: Exclude<keyof NonNullable<AdvancedCustomRoute['auth']>, 'type'>,
    value: string
  ) => {
    const currentAuth = route.auth
    if (!currentAuth || currentAuth.type === 'none') return
    onChange({
      auth: {
        type: currentAuth.type as AdvancedCustomAuthType,
        name: currentAuth.name || '',
        value: currentAuth.value || '',
        [field]: value,
      },
    })
  }

  return (
    <div
      className={cn(
        'flex flex-col gap-4 px-4 py-4 lg:gap-2 lg:px-3 lg:py-3',
        errorMessage && 'bg-destructive/5'
      )}
    >
      <div
        className={cn(
          'grid gap-4 md:grid-cols-2 lg:items-center lg:gap-2',
          routeEditorGridClassName
        )}
      >
        <div className='flex min-w-0 items-start justify-between gap-3 md:col-span-2 lg:col-span-1'>
          <div className='min-w-0 space-y-2 lg:space-y-1'>
            <div className='flex flex-wrap items-center gap-2'>
              <div className='text-sm font-medium'>
                {t('Route')} {index + 1}
              </div>
              {isModelListRoute ? (
                <Badge variant='outline'>
                  {ADVANCED_CUSTOM_MODEL_LIST_LABEL}
                </Badge>
              ) : null}
              {!isModelListRoute && isFallback ? (
                <Badge variant='outline'>{t('Fallback')}</Badge>
              ) : null}
              <TooltipProvider delay={100}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span
                        className={cn(
                          'border-border inline-flex size-7 shrink-0 items-center justify-center rounded-md border',
                          isNativeConverter
                            ? 'bg-secondary text-secondary-foreground'
                            : 'bg-muted text-foreground'
                        )}
                      />
                    }
                  >
                    <ConverterVisualIcon
                      className='size-3.5'
                      aria-hidden='true'
                    />
                    <span className='sr-only'>{t(converterLabel)}</span>
                  </TooltipTrigger>
                  <TooltipContent side='top'>
                    {t(converterLabel)}
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </div>
          <div className='flex shrink-0 items-center gap-1 lg:hidden'>
            <TooltipIconButton
              label={t('Move route up')}
              icon={ArrowUp}
              disabled={!canMoveUp}
              onClick={onMoveUp}
            />
            <TooltipIconButton
              label={t('Move route down')}
              icon={ArrowDown}
              disabled={!canMoveDown}
              onClick={onMoveDown}
            />
            {catchAllOutOfOrder ? (
              <TooltipIconButton
                label={t('Move fallback to end')}
                icon={ArrowDownToLine}
                onClick={onMoveCatchAllToEnd}
              />
            ) : null}
            <TooltipIconButton
              label={t('Delete')}
              icon={Trash2}
              onClick={onRemove}
            />
          </div>
        </div>

        <FieldBlock
          label={
            <span className='inline-flex items-center gap-1'>
              {t('Client model')}
              <ModelRuleHelpPopover />
            </span>
          }
          className='lg:gap-1'
          labelClassName='lg:sr-only'
        >
          {isModelListRoute && parsedRouteModels.length === 0 ? (
            <div className='flex h-9 items-center'>
              <Badge variant='outline'>
                {ADVANCED_CUSTOM_MODEL_LIST_LABEL}
              </Badge>
            </div>
          ) : (
            <>
              <Input
                value={modelsInputValue}
                onChange={(event) => setModelsInput(event.target.value)}
                onBlur={(event) => normalizeModelsInput(event.target.value)}
                placeholder={
                  isFallback
                    ? t('Leave empty for fallback')
                    : t('e.g. gpt-4o, gemini-2.5-flash')
                }
                aria-invalid={Boolean(errorMessage)}
              />
              <div className='flex flex-wrap gap-1'>
                {isFallback ? (
                  <Badge variant='outline'>{t('Fallback')}</Badge>
                ) : (
                  parsedRouteModels.map((model) => {
                    const ruleKind = getAdvancedCustomModelRuleKind(model)
                    const displayModel =
                      ruleKind === 'regex'
                        ? getAdvancedCustomRegexModelPattern(model) || model
                        : model
                    return (
                      <Badge
                        key={model}
                        variant={ruleKind === 'regex' ? 'outline' : 'secondary'}
                        className='max-w-full gap-1.5 font-mono'
                      >
                        <span className='font-sans text-[10px] font-semibold tracking-normal uppercase'>
                          {t(ruleKind === 'regex' ? 'Regex' : 'Exact')}
                        </span>
                        <span className='truncate'>{displayModel}</span>
                      </Badge>
                    )
                  })
                )}
              </div>
            </>
          )}
        </FieldBlock>

        <FieldBlock
          label={t('Upstream path')}
          className='lg:gap-1'
          labelClassName='lg:sr-only'
        >
          <Input
            value={route.upstream_path || ''}
            onChange={(event) =>
              onChange({
                upstream_path: event.target.value,
              })
            }
            placeholder={getAdvancedCustomUpstreamPathPlaceholder(
              converter,
              incomingPath
            )}
          />
          <p className='text-muted-foreground text-xs leading-relaxed lg:hidden'>
            {t(upstreamPathDescriptionKey)}
          </p>
        </FieldBlock>

        <FieldBlock
          label={t('Converter')}
          className='lg:gap-1'
          labelClassName='lg:sr-only'
        >
          <Select
            value={converter}
            disabled={isModelListRoute && converter === 'none'}
            onValueChange={(value) =>
              setConverter(value as AdvancedCustomConverter)
            }
          >
            <SelectTrigger className='w-full max-w-full lg:h-8'>
              <SelectValue className='min-w-0 truncate'>
                {t(converterTriggerLabel)}
              </SelectValue>
            </SelectTrigger>
            <SelectContent
              alignItemWithTrigger={false}
              className={longSelectContentClass}
            >
              <SelectGroup>
                {converterOptions.map((option) => (
                  <SelectItem
                    key={option.value}
                    value={option.value}
                    className={longSelectItemClass}
                  >
                    <span className='min-w-0 leading-snug break-words whitespace-normal'>
                      {t(option.label)}
                    </span>
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </FieldBlock>

        <FieldBlock
          label={t('Auth')}
          className='lg:gap-1'
          labelClassName='lg:sr-only'
        >
          <Select
            value={authMode}
            onValueChange={(value) =>
              setAuthMode(value as AdvancedCustomAuthMode)
            }
          >
            <SelectTrigger className='w-full max-w-full lg:h-8'>
              <SelectValue className='min-w-0 truncate'>
                {t(authLabel)}
              </SelectValue>
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {ADVANCED_CUSTOM_AUTH_MODE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {t(option.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </FieldBlock>

        <div className='hidden items-center justify-end gap-1 lg:flex'>
          <TooltipIconButton
            label={t('Move route up')}
            icon={ArrowUp}
            disabled={!canMoveUp}
            onClick={onMoveUp}
          />
          <TooltipIconButton
            label={t('Move route down')}
            icon={ArrowDown}
            disabled={!canMoveDown}
            onClick={onMoveDown}
          />
          {catchAllOutOfOrder ? (
            <TooltipIconButton
              label={t('Move fallback to end')}
              icon={ArrowDownToLine}
              onClick={onMoveCatchAllToEnd}
            />
          ) : null}
          <TooltipIconButton
            label={t('Delete')}
            icon={Trash2}
            onClick={onRemove}
          />
        </div>
      </div>

      {errorMessage ? (
        <p className='text-destructive text-xs'>{t(errorMessage)}</p>
      ) : null}

      {supportsResponsesToolHandling ? (
        <>
          <Separator className='lg:hidden' />
          <div
            className={cn(
              'grid gap-4 md:grid-cols-2 lg:items-end lg:gap-2 lg:border-t lg:pt-2',
              routeEditorGridClassName
            )}
          >
            <span className='hidden lg:block' aria-hidden='true' />
            <FieldBlock
              label={t('Responses tools')}
              className='lg:gap-1'
              labelClassName='lg:text-xs'
            >
              <Select
                value={responsesToolsMode}
                onValueChange={(value) =>
                  setResponsesToolsMode(
                    value as AdvancedCustomResponsesToolsMode
                  )
                }
              >
                <SelectTrigger className='w-full max-w-full lg:h-8'>
                  <SelectValue className='min-w-0 truncate'>
                    {t(responsesToolsModeLabel)}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent
                  alignItemWithTrigger={false}
                  className={longSelectContentClass}
                >
                  <SelectGroup>
                    {ADVANCED_CUSTOM_RESPONSES_TOOLS_MODE_OPTIONS.map(
                      (option) => (
                        <SelectItem
                          key={option.value}
                          value={option.value}
                          className={longSelectItemClass}
                        >
                          <div className='flex min-w-0 flex-col gap-1 leading-snug whitespace-normal'>
                            <span>{t(option.label)}</span>
                            <span className='text-muted-foreground text-xs'>
                              {t(option.description)}
                            </span>
                          </div>
                        </SelectItem>
                      )
                    )}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </FieldBlock>
            <span className='hidden lg:block' aria-hidden='true' />
            <span className='hidden lg:block' aria-hidden='true' />
            <span className='hidden lg:block' aria-hidden='true' />
            <span className='hidden lg:block' aria-hidden='true' />
          </div>
          <div
            className={cn(
              'grid gap-4 md:grid-cols-2 lg:items-end lg:gap-2 lg:border-t lg:pt-2',
              responseToolPolicyGridClassName
            )}
          >
            <span className='hidden lg:block' aria-hidden='true' />
            {responseToolPolicyFields.map((field) => (
              <FieldBlock
                key={field.key}
                label={t(field.label)}
                className='lg:gap-1'
                labelClassName='lg:text-xs'
              >
                <Select
                  value={responseToolPolicies[field.key]}
                  onValueChange={(value) =>
                    setResponseToolPolicy(
                      field.key,
                      value as AdvancedCustomResponsesToolPolicy
                    )
                  }
                >
                  <SelectTrigger className='w-full max-w-full lg:h-8'>
                    <SelectValue className='min-w-0 truncate'>
                      {t(responseToolPolicies[field.key] || 'preserve')}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent
                    alignItemWithTrigger={false}
                    className={longSelectContentClass}
                  >
                    <SelectGroup>
                      {ADVANCED_CUSTOM_RESPONSES_TOOL_POLICY_OPTIONS.filter(
                        (option) =>
                          (!isNativeConverter && field.allowFlatten) ||
                          option.value !== 'flatten'
                      ).map((option) => (
                        <SelectItem
                          key={option.value}
                          value={option.value}
                          className={longSelectItemClass}
                        >
                          <div className='flex min-w-0 flex-col gap-1 leading-snug whitespace-normal'>
                            <span>{t(option.label)}</span>
                            <span className='text-muted-foreground text-xs'>
                              {t(option.description)}
                            </span>
                          </div>
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </FieldBlock>
            ))}
          </div>
          <div className='space-y-3 border-t pt-3'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div>
                <p className='text-sm font-medium'>{t('Tool Handling')}</p>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Function tools are always preserved. Model overrides take precedence over route policies.'
                  )}
                </p>
              </div>
              <Select
                value={responseToolConflictPolicy}
                onValueChange={(value) =>
                  patchConverterOptions({
                    responses_tool_conflict_policy:
                      value as AdvancedCustomResponsesToolConflictPolicy,
                  })
                }
              >
                <SelectTrigger className='w-40'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='deduplicate'>
                    {t('Deduplicate conflicts')}
                  </SelectItem>
                  <SelectItem value='preserve'>
                    {t('Preserve conflicts')}
                  </SelectItem>
                  <SelectItem value='reject'>
                    {t('Reject conflicts')}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-2'>
              {staleOverrideModels.length > 0 ? (
                <Alert variant='destructive'>
                  <AlertDescription className='break-words'>
                    {t(
                      'Stale model overrides are not present in the channel model list: {{models}}',
                      { models: staleOverrideModels.join(', ') }
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}
              {modelToolOverrides.map((override, overrideIndex) => (
                <div
                  key={override.models?.join(',') || JSON.stringify(override)}
                  className='space-y-2 rounded border p-2'
                >
                  <div className='flex gap-2'>
                    <Input
                      value={(override.models || []).join(', ')}
                      onChange={(event) =>
                        setModelToolOverride(overrideIndex, {
                          models: [
                            ...new Set(
                              event.target.value
                                .split(',')
                                .map((model) => model.trim())
                                .filter(Boolean)
                            ),
                          ],
                        })
                      }
                      placeholder={t('Models, comma separated')}
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      onClick={() => removeModelToolOverride(overrideIndex)}
                    >
                      <Trash2 className='h-4 w-4' />
                      <span className='sr-only'>{t('Delete')}</span>
                    </Button>
                  </div>
                  <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3'>
                    {responseToolPolicyFields.map((field) => (
                      <FieldBlock
                        key={field.key}
                        label={t(field.label)}
                        labelClassName='text-xs'
                      >
                        <Select
                          value={
                            override.responses_tools?.[field.key] || 'inherit'
                          }
                          onValueChange={(value) =>
                            setModelToolOverridePolicy(
                              overrideIndex,
                              field.key,
                              value as
                                | AdvancedCustomResponsesToolPolicy
                                | 'inherit'
                            )
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value='inherit'>
                              {t('Inherit route policy')}:{' '}
                              {
                                resolveAdvancedCustomResponsesToolPolicy(
                                  {
                                    ...route,
                                    converter_options: {
                                      ...route.converter_options,
                                      responses_tool_model_overrides: [],
                                    },
                                  },
                                  '',
                                  field.key
                                ).policy
                              }
                            </SelectItem>
                            {ADVANCED_CUSTOM_RESPONSES_TOOL_POLICY_OPTIONS.filter(
                              (option) =>
                                (!isNativeConverter && field.allowFlatten) ||
                                option.value !== 'flatten'
                            ).map((option) => (
                              <SelectItem
                                key={option.value}
                                value={option.value}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </FieldBlock>
                    ))}
                  </div>
                  {(override.responses_tool_names || []).map(
                    (namePolicy, policyIndex) => (
                      <div
                        key={`${namePolicy.tool_type || 'tool'}:${namePolicy.tool_name || namePolicy.policy || 'policy'}`}
                        className='grid gap-2 sm:grid-cols-[9rem_1fr_9rem_2rem]'
                      >
                        <Input
                          value={namePolicy.tool_type || ''}
                          onChange={(event) =>
                            setNamePolicy(overrideIndex, policyIndex, {
                              tool_type: event.target.value,
                            })
                          }
                          placeholder={t('Tool type')}
                        />
                        <Input
                          value={namePolicy.tool_name || ''}
                          onChange={(event) =>
                            setNamePolicy(overrideIndex, policyIndex, {
                              tool_name: event.target.value,
                            })
                          }
                          placeholder={t('Tool name')}
                        />
                        <Select
                          value={namePolicy.policy || 'preserve'}
                          onValueChange={(value) =>
                            setNamePolicy(overrideIndex, policyIndex, {
                              policy:
                                value as AdvancedCustomResponsesToolPolicy,
                            })
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {ADVANCED_CUSTOM_RESPONSES_TOOL_POLICY_OPTIONS.filter(
                              (option) =>
                                !isNativeConverter || option.value !== 'flatten'
                            ).map((option) => (
                              <SelectItem
                                key={option.value}
                                value={option.value}
                              >
                                {t(option.label)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          onClick={() =>
                            removeNamePolicy(overrideIndex, policyIndex)
                          }
                        >
                          <Trash2 className='h-4 w-4' />
                          <span className='sr-only'>{t('Delete')}</span>
                        </Button>
                      </div>
                    )
                  )}
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() => addNamePolicy(overrideIndex)}
                  >
                    <Plus className='mr-1 h-3.5 w-3.5' />
                    {t('Add tool name rule')}
                  </Button>
                </div>
              ))}
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={addModelToolOverride}
              >
                <Plus className='mr-1 h-3.5 w-3.5' />
                {t('Add model capability')}
              </Button>
            </div>
          </div>
          {!isNativeConverter ? (
            <div
              className={cn(
                'grid gap-4 md:grid-cols-2 lg:items-end lg:gap-2 lg:border-t lg:pt-2',
                routeEditorGridClassName
              )}
            >
              <span className='hidden lg:block' aria-hidden='true' />
              <FieldBlock
                label={t('Drop Responses fields')}
                description={t(
                  'Strip these Responses request fields before forwarding to the chat-only upstream.'
                )}
                className='lg:gap-1'
                labelClassName='lg:text-xs'
              >
                <div className='grid grid-cols-1 gap-1.5 text-xs sm:grid-cols-2'>
                  {ADVANCED_CUSTOM_RESPONSES_DROP_FIELDS.map((option) => {
                    const checked = responsesDropFields.includes(option.value)
                    return (
                      <label
                        key={option.value}
                        className='border-border/60 hover:bg-muted/40 flex cursor-pointer items-start gap-2 rounded border px-2 py-1.5'
                        title={`${option.label} — ${option.description}`}
                      >
                        <input
                          type='checkbox'
                          className='mt-0.5 h-3.5 w-3.5 shrink-0 accent-current'
                          checked={checked}
                          onChange={(event) =>
                            setResponsesDropField(
                              option.value,
                              event.target.checked
                            )
                          }
                        />
                        <span className='flex min-w-0 flex-col leading-snug'>
                          <span className='truncate font-medium'>
                            {option.label}
                          </span>
                          <span className='text-muted-foreground truncate text-[11px]'>
                            {option.description}
                          </span>
                        </span>
                      </label>
                    )
                  })}
                </div>
              </FieldBlock>
            </div>
          ) : null}
        </>
      ) : null}

      {authMode === 'header' || authMode === 'query' ? (
        <>
          <Separator className='lg:hidden' />
          <div
            className={cn(
              'grid gap-4 md:grid-cols-2 lg:items-end lg:gap-2 lg:border-t lg:pt-2',
              routeEditorGridClassName
            )}
          >
            <span className='hidden lg:block' aria-hidden='true' />
            <FieldBlock
              label={t('Auth name')}
              className='lg:gap-1'
              labelClassName='lg:text-xs'
            >
              <Input
                value={route.auth?.name || ''}
                onChange={(event) => updateAuth('name', event.target.value)}
                placeholder={
                  authMode === 'header' ? 'Authorization' : 'api_key'
                }
              />
            </FieldBlock>
            <FieldBlock
              label={t('Auth value')}
              className='lg:gap-1'
              labelClassName='lg:text-xs'
            >
              <Input
                value={route.auth?.value || ''}
                onChange={(event) => updateAuth('value', event.target.value)}
                placeholder={
                  authMode === 'header' ? 'Bearer {api_key}' : '{api_key}'
                }
              />
            </FieldBlock>
            <span className='hidden lg:block' aria-hidden='true' />
            <span className='hidden lg:block' aria-hidden='true' />
            <span className='hidden lg:block' aria-hidden='true' />
          </div>
        </>
      ) : null}
    </div>
  )
}

function ModelRuleHelpPopover() {
  const { t } = useTranslation()

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='text-muted-foreground hover:text-foreground size-6'
            aria-label={t('Client model matching help')}
          />
        }
      >
        <Info className='size-3.5' aria-hidden='true' />
      </PopoverTrigger>
      <PopoverContent
        align='start'
        side='bottom'
        sideOffset={8}
        className='w-[min(22rem,calc(100vw-2rem))] gap-3 p-3'
      >
        <PopoverHeader className='gap-1'>
          <PopoverTitle>{t('Client model matching')}</PopoverTitle>
          <PopoverDescription className='text-xs leading-relaxed'>
            {t(
              'Rules match the original model value from the client request body.'
            )}
          </PopoverDescription>
        </PopoverHeader>
        <div className='text-muted-foreground space-y-2 text-xs leading-relaxed'>
          <p>
            {t(
              'Use exact model names such as gpt-4o, or regex rules prefixed with re: such as re:^gemini-.'
            )}
          </p>
          <p>
            {t(
              'Separate multiple rules with English commas. For regex patterns that need commas, switch to JSON Text.'
            )}
          </p>
          <p>
            {t(
              'Leave the final split empty as the fallback for models not matched above.'
            )}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function TooltipIconButton({
  label,
  icon: Icon,
  disabled,
  onClick,
}: {
  label: string
  icon: LucideIcon
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <TooltipProvider delay={100}>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type='button'
              variant='ghost'
              size='icon'
              disabled={disabled}
              onClick={onClick}
            />
          }
        >
          <Icon data-icon='inline-start' aria-hidden='true' />
          <span className='sr-only'>{label}</span>
        </TooltipTrigger>
        <TooltipContent side='top'>{label}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function FieldBlock({
  label,
  description,
  className,
  labelClassName,
  children,
}: {
  label: ReactNode
  description?: string
  className?: string
  labelClassName?: string
  children: ReactNode
}) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-2', className)}>
      <span
        className={cn('text-sm font-medium', labelClassName)}
        title={typeof label === 'string' ? label : undefined}
      >
        {label}
      </span>
      {description ? (
        <span className='text-muted-foreground text-[11px] leading-snug'>
          {description}
        </span>
      ) : null}
      {children}
    </div>
  )
}

function DualEndpointQuickSetup({
  onApply,
}: {
  onApply: (config: AdvancedCustomConfig) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [anthropicBaseURL, setAnthropicBaseURL] = useState('')
  const [openaiBaseURL, setOpenAIBaseURL] = useState('')
  const [enableAnthropic, setEnableAnthropic] = useState(true)
  const [enableOpenAIChat, setEnableOpenAIChat] = useState(true)
  const [enableOpenAIResponses, setEnableOpenAIResponses] = useState(false)
  const [anthropicAuth, setAnthropicAuth] = useState<'bearer' | 'x-api-key'>(
    'x-api-key'
  )
  const [openaiAuth, setOpenaiAuth] = useState<'bearer' | 'x-api-key'>('bearer')
  const [error, setError] = useState('')

  const handleApply = () => {
    setError('')
    if (enableAnthropic && !anthropicBaseURL.trim()) {
      setError(t('Anthropic-compatible Base URL is required'))
      return
    }
    if ((enableOpenAIChat || enableOpenAIResponses) && !openaiBaseURL.trim()) {
      setError(t('OpenAI-compatible Base URL is required'))
      return
    }
    if (!enableAnthropic && !enableOpenAIChat && !enableOpenAIResponses) {
      setError(t('Enable at least one endpoint'))
      return
    }
    const generated = createDualEndpointConfig({
      anthropicBaseURL: anthropicBaseURL.trim(),
      openaiBaseURL: openaiBaseURL.trim(),
      enableAnthropicMessages: enableAnthropic,
      enableOpenAIChat,
      enableOpenAIResponses,
      anthropicAuthMode: anthropicAuth,
      openaiAuthMode: openaiAuth,
    })
    onApply(generated)
    setOpen(false)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) {
      setError('')
    }
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={<Button type='button' variant='outline' size='sm' />}
      >
        {t('Quick Setup')}
      </PopoverTrigger>
      <PopoverContent align='end' className='w-[420px] p-4' sideOffset={8}>
        <div className='flex flex-col gap-3'>
          <div className='flex flex-col gap-1'>
            <div className='text-sm font-medium'>
              {t('Dual-endpoint quick setup')}
            </div>
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                'Enter the Anthropic-compatible and OpenAI-compatible base URLs. Routes are generated automatically.'
              )}
            </p>
          </div>

          <div className='flex flex-col gap-1.5'>
            <Label htmlFor='qs-anthropic-base'>
              {t('Anthropic-compatible Base URL')}
            </Label>
            <Input
              id='qs-anthropic-base'
              value={anthropicBaseURL}
              onChange={(event) => setAnthropicBaseURL(event.target.value)}
              placeholder='https://api.example.com/anthropic'
              disabled={!enableAnthropic}
            />
          </div>

          <div className='flex flex-col gap-1.5'>
            <Label htmlFor='qs-openai-base'>
              {t('OpenAI-compatible Base URL')}
            </Label>
            <Input
              id='qs-openai-base'
              value={openaiBaseURL}
              onChange={(event) => setOpenAIBaseURL(event.target.value)}
              placeholder='https://api.example.com/openai'
              disabled={!enableOpenAIChat && !enableOpenAIResponses}
            />
          </div>

          <div className='flex flex-col gap-1.5'>
            <Label className='text-xs font-medium'>
              {t('Endpoints to enable')}
            </Label>
            <div className='flex flex-col gap-1.5'>
              <label className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={enableAnthropic}
                  onCheckedChange={(value) =>
                    setEnableAnthropic(value === true)
                  }
                />
                <span>{t('Claude Messages (/v1/messages)')}</span>
              </label>
              <label className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={enableOpenAIChat}
                  onCheckedChange={(value) =>
                    setEnableOpenAIChat(value === true)
                  }
                />
                <span>
                  {t('OpenAI Chat Completions (/v1/chat/completions)')}
                </span>
              </label>
              <label className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={enableOpenAIResponses}
                  onCheckedChange={(value) =>
                    setEnableOpenAIResponses(value === true)
                  }
                />
                <span>{t('OpenAI Responses (/v1/responses)')}</span>
              </label>
            </div>
          </div>

          <div className='grid grid-cols-2 gap-3'>
            <div className='flex flex-col gap-1.5'>
              <Label className='text-xs font-medium'>
                {t('Anthropic auth')}
              </Label>
              <Select
                value={anthropicAuth}
                onValueChange={(value) =>
                  setAnthropicAuth(value as 'bearer' | 'x-api-key')
                }
              >
                <SelectTrigger className='h-8 w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='x-api-key'>x-api-key</SelectItem>
                  <SelectItem value='bearer'>Bearer</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='flex flex-col gap-1.5'>
              <Label className='text-xs font-medium'>{t('OpenAI auth')}</Label>
              <Select
                value={openaiAuth}
                onValueChange={(value) =>
                  setOpenaiAuth(value as 'bearer' | 'x-api-key')
                }
              >
                <SelectTrigger className='h-8 w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='bearer'>Bearer</SelectItem>
                  <SelectItem value='x-api-key'>x-api-key</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {error ? <p className='text-destructive text-xs'>{error}</p> : null}

          <div className='flex justify-end gap-2 pt-1'>
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => handleOpenChange(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='button' size='sm' onClick={handleApply}>
              {t('Apply')}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
