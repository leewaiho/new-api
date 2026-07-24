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
import { ArrowRight, Check, Plus, Shuffle, Trash2 } from 'lucide-react'
import { type ReactNode, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
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
import { Textarea } from '@/components/ui/textarea'
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
  ADVANCED_CUSTOM_IMPLICIT_HOSTED_TOOL_OPTIONS,
  ADVANCED_CUSTOM_INCOMING_PATH_OPTIONS,
  ADVANCED_CUSTOM_RESPONSES_TOOL_POLICY_OPTIONS,
  ADVANCED_CUSTOM_RESPONSES_TOOLS_MODE_OPTIONS,
  ADVANCED_CUSTOM_TEMPLATE_OPTIONS,
  type AdvancedCustomAuthMode,
  buildAdvancedCustomAuth,
  createAdvancedCustomConfig,
  createAdvancedCustomRoute,
  createDualEndpointConfig,
  getAdvancedCustomAuthMode,
  getAdvancedCustomConverterOptions,
  getAdvancedCustomIncomingPathLabel,
  ADVANCED_CUSTOM_RESPONSES_DROP_FIELDS,
  type AdvancedCustomResponsesDropField,
  type AdvancedCustomResponsesToolType,
  getAdvancedCustomTemplateConfig,
  getAdvancedCustomUpstreamPathPlaceholder,
  getDefaultAdvancedCustomIncomingPath,
  isAdvancedCustomIncomingPathAllowed,
  mergeAdvancedCustomRouteCompatibilityConfig,
  normalizeAdvancedCustomConfig,
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
  AdvancedCustomResponsesToolMissingOptionsPolicy,
  AdvancedCustomResponsesToolModelOverride,
  AdvancedCustomWebSearchParameterDefaults,
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
  'lg:grid-cols-[7rem_minmax(0,1.45fr)_minmax(0,1.35fr)_minmax(0,1fr)_minmax(0,0.85fr)_2rem]'
const responseToolPolicyGridClassName =
  'lg:grid-cols-[7rem_repeat(6,minmax(0,1fr))]'
const upstreamPathDescriptionKey =
  'Use a path to append it to the channel Base URL, or enter a full URL to override the Base URL for this route.'

function getOptionLabel(
  options: ReadonlyArray<{ value: string; label: string }>,
  value: string
) {
  return options.find((option) => option.value === value)?.label || value
}

function modelFetchURLsToText(urls: string[] | undefined): string {
  return (urls || []).join('\n')
}

function getImplicitHostedToolsSource(
  routeTools: string[],
  modelTools: string[]
): 'system default (none)' | 'route' | 'model' | 'route + model' {
  if (modelTools.length > 0) {
    return routeTools.length > 0 ? 'route + model' : 'model'
  }
  return routeTools.length > 0 ? 'route' : 'system default (none)'
}

function updateWebSearchParameterDefaults(
  current: AdvancedCustomWebSearchParameterDefaults | undefined,
  key: 'enable' | 'search_result' | 'search_engine',
  value: boolean | string | undefined
): AdvancedCustomWebSearchParameterDefaults {
  const defaults = { ...current }
  switch (key) {
    case 'enable':
      if (typeof value === 'boolean') defaults.enable = value
      else delete defaults.enable
      break
    case 'search_result':
      if (typeof value === 'boolean') defaults.search_result = value
      else delete defaults.search_result
      break
    case 'search_engine':
      if (typeof value === 'string' && value !== '') {
        defaults.search_engine = value
      } else {
        delete defaults.search_engine
      }
      break
  }
  return defaults
}

function textToModelFetchURLs(value: string): string[] {
  return [
    ...new Set(
      value
        .split(/[\n,]+/)
        .map((url) => url.trim())
        .filter(Boolean)
    ),
  ]
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
  const routes = normalizedConfig.advanced_routes || []
  const routeRows = routes.map((route, index) => ({
    route,
    routeKey:
      routeKeys.at(index) ||
      route.incoming_path ||
      route.upstream_path ||
      route.converter ||
      'advanced-custom-route',
  }))
  const validationError = useMemo(
    () => validateAdvancedCustomConfig(normalizedConfig),
    [normalizedConfig]
  )

  const createRouteKey = () => {
    routeKeyCounterRef.current += 1
    return `advanced-custom-route-${routeKeyCounterRef.current}`
  }

  const createRouteKeys = (count: number) =>
    Array.from({ length: count }, () => createRouteKey())

  const updateModelFetchURLs = (value: string) => {
    const modelFetchURLs = textToModelFetchURLs(value)
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return {
        ...next,
        ...(modelFetchURLs.length > 0
          ? { model_fetch_urls: modelFetchURLs }
          : { model_fetch_urls: undefined }),
      }
    })
  }

  const updateRoute = (index: number, patch: Partial<AdvancedCustomRoute>) => {
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      const nextRoutes = [...(next.advanced_routes || [])]
      nextRoutes[index] = { ...nextRoutes[index], ...patch }
      return { ...next, advanced_routes: nextRoutes }
    })
  }

  const addRoute = () => {
    setConfig((current) => {
      const next = normalizeAdvancedCustomConfig(current)
      return {
        ...next,
        advanced_routes: [
          ...(next.advanced_routes || []),
          createAdvancedCustomRoute(),
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

  const formatJson = () => {
    const parsed = parseJsonEditorConfig()
    if (!parsed) return
    setJsonText(stringifyAdvancedCustomConfig(parsed))
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
        ...(base.model_fetch_urls?.length
          ? { model_fetch_urls: base.model_fetch_urls }
          : {}),
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
            <Check className='mr-2 h-4 w-4' />
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
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={addRoute}
            >
              <Plus className='mr-2 h-4 w-4' />
              {t('Add route')}
            </Button>
          </div>

          {validationError ? (
            <Alert variant='destructive'>
              <AlertDescription>
                {validationError.routeIndex !== undefined
                  ? `${t('Route')} ${validationError.routeIndex + 1}: `
                  : ''}
                {t(validationError.message)}
              </AlertDescription>
            </Alert>
          ) : null}

          <p className='text-muted-foreground bg-muted/30 hidden rounded-md border px-3 py-2 text-xs leading-relaxed lg:block'>
            {t(upstreamPathDescriptionKey)}
          </p>

          <div className='space-y-2 rounded-md border p-3'>
            <Label>{t('Model fetch URLs')}</Label>
            <Textarea
              value={modelFetchURLsToText(normalizedConfig.model_fetch_urls)}
              onChange={(event) => updateModelFetchURLs(event.target.value)}
              placeholder='https://api.example.com/v1/models'
              className='min-h-20 font-mono text-xs'
            />
            <p className='text-muted-foreground text-xs leading-relaxed'>
              {t(
                'One model list endpoint per line. Leave empty to infer from OpenAI-compatible routes.'
              )}
            </p>
          </div>

          <div className='flex flex-col gap-4 lg:gap-2'>
            <div
              className={cn(
                'text-muted-foreground hidden items-center gap-2 px-3 text-xs font-medium lg:grid',
                routeEditorGridClassName
              )}
            >
              <span>{t('Route')}</span>
              <span>{t('Incoming path')}</span>
              <span>{t('Upstream path')}</span>
              <span>{t('Converter')}</span>
              <span>{t('Auth')}</span>
              <span aria-hidden='true' />
            </div>
            {routeRows.map((routeRow, index) => (
              <RouteEditor
                key={routeRow.routeKey}
                route={routeRow.route}
                index={index}
                onChange={(patch) => updateRoute(index, patch)}
                onRemove={() => removeRoute(index)}
                channelModels={channelModels || []}
              />
            ))}
          </div>
        </div>
      ) : (
        <div className='p-4'>
          <div className='mb-2 flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={formatJson}
            >
              {t('Format')}
            </Button>
            <span className='text-muted-foreground text-xs'>
              {t('Advanced text editing')}
            </span>
          </div>
          <Textarea
            value={jsonText}
            onChange={(event) => handleJsonChange(event.target.value)}
            placeholder={stringifyAdvancedCustomConfig(
              getAdvancedCustomTemplateConfig(templateKey)
            )}
            rows={22}
            className='min-h-[420px] font-mono text-xs'
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

const responseToolPolicyFields: Array<{
  key: Exclude<AdvancedCustomResponsesToolType, 'function'>
  label: string
  allowFlatten: boolean
}> = [
  { key: 'namespace', label: 'Namespace', allowFlatten: true },
  { key: 'custom', label: 'Custom', allowFlatten: true },
  { key: 'web_search', label: 'Web search', allowFlatten: false },
  { key: 'tool_search', label: 'Tool search', allowFlatten: true },
  { key: 'image_generation', label: 'Image generation', allowFlatten: false },
  { key: 'unknown', label: 'Unknown / other', allowFlatten: false },
]

function RouteEditor({
  route,
  index,
  onChange,
  onRemove,
  channelModels,
}: {
  route: AdvancedCustomRoute
  index: number
  onChange: (patch: Partial<AdvancedCustomRoute>) => void
  onRemove: () => void
  channelModels: string[]
}) {
  const { t } = useTranslation()
  const converter = route.converter || 'none'
  const authMode = getAdvancedCustomAuthMode(route)
  const incomingPath =
    route.incoming_path || getDefaultAdvancedCustomIncomingPath(converter)
  const converterOptions = useMemo(
    () => getAdvancedCustomConverterOptions(incomingPath),
    [incomingPath]
  )
  const incomingPathLabel = getAdvancedCustomIncomingPathLabel(incomingPath)
  const converterLabel = getOptionLabel(
    ADVANCED_CUSTOM_CONVERTER_OPTIONS,
    converter
  )
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

  const isNativeConverter = converter === 'none'
  const isResponsesRoute =
    incomingPath === '/v1/responses' || incomingPath === '/v1/responses/compact'
  const supportsResponsesToolHandling =
    isResponsesRoute &&
    (isNativeConverter ||
      converter === 'openai_responses_to_openai_chat_completions')
  const ConverterVisualIcon = isNativeConverter ? ArrowRight : Shuffle

  const setConverter = (nextConverter: AdvancedCustomConverter) => {
    const patch: Partial<AdvancedCustomRoute> = { converter: nextConverter }
    if (!isAdvancedCustomIncomingPathAllowed(incomingPath, nextConverter)) {
      patch.incoming_path = getDefaultAdvancedCustomIncomingPath(nextConverter)
    }
    if (
      nextConverter !== 'none' &&
      nextConverter !== 'openai_responses_to_openai_chat_completions'
    ) {
      patch.converter_options = undefined
    } else if (!route.converter_options?.responses_tools_mode) {
      patch.converter_options = {
        responses_tools_mode:
          nextConverter === 'none' ? 'preserve' : 'compat_flatten',
      }
    }
    onChange(patch)
  }

  const setIncomingPath = (nextIncomingPath: string | null) => {
    const resolvedIncomingPath =
      nextIncomingPath || getDefaultAdvancedCustomIncomingPath(converter)
    const patch: Partial<AdvancedCustomRoute> = {
      incoming_path: resolvedIncomingPath,
    }
    if (!isAdvancedCustomIncomingPathAllowed(resolvedIncomingPath, converter)) {
      patch.converter = 'none'
    }
    onChange(patch)
  }

  const setAuthMode = (mode: AdvancedCustomAuthMode) => {
    onChange({ auth: buildAdvancedCustomAuth(mode, route.auth) })
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
  const routeImplicitHostedTools = useMemo(
    () => route.converter_options?.responses_implicit_hosted_tools || [],
    [route.converter_options?.responses_implicit_hosted_tools]
  )
  const routeWebSearchParameters =
    route.converter_options?.responses_tool_parameters?.web_search
  const routeWebSearchStatus = (() => {
    if (route.converter_options?.responses_tools?.web_search === 'drop') {
      return 'Hosted web_search is dropped; client function tools are preserved'
    }
    if (
      routeWebSearchParameters?.when_nested_options_missing ===
      'populate_defaults'
    ) {
      return 'Enabled: schema-only parameter filling'
    }
    return 'Disabled: pass through unchanged'
  })()

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
  const setRouteImplicitHostedTool = (capability: string, enabled: boolean) => {
    const next = new Set(routeImplicitHostedTools)
    if (enabled) next.add(capability)
    else next.delete(capability)
    patchConverterOptions({ responses_implicit_hosted_tools: [...next] })
  }
  const setRouteWebSearchMode = (
    mode: AdvancedCustomResponsesToolMissingOptionsPolicy
  ) => {
    patchConverterOptions({
      responses_tool_parameters: {
        web_search:
          mode === 'populate_defaults'
            ? {
                when_nested_options_missing: mode,
                defaults: routeWebSearchParameters?.defaults || {
                  enable: true,
                  search_result: true,
                },
              }
            : { when_nested_options_missing: mode },
      },
    })
  }
  const applyRouteClientWebSearchFallback = () => {
    patchConverterOptions({
      responses_tools: {
        ...route.converter_options?.responses_tools,
        web_search: 'drop',
      },
      responses_tool_parameters: undefined,
    })
  }
  const setRouteWebSearchDefault = (
    key: 'enable' | 'search_result' | 'search_engine',
    value: boolean | string | undefined
  ) => {
    const defaults = updateWebSearchParameterDefaults(
      routeWebSearchParameters?.defaults,
      key,
      value
    )
    patchConverterOptions({
      responses_tool_parameters: {
        web_search: {
          when_nested_options_missing: 'populate_defaults',
          defaults,
        },
      },
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
  const setModelImplicitHostedTool = (
    index: number,
    capability: string,
    enabled: boolean
  ) => {
    const next = new Set(
      modelToolOverrides[index].responses_implicit_hosted_tools || []
    )
    if (enabled) next.add(capability)
    else next.delete(capability)
    setModelToolOverride(index, {
      responses_implicit_hosted_tools: [...next],
    })
  }
  const setModelWebSearchMode = (
    index: number,
    mode: AdvancedCustomResponsesToolMissingOptionsPolicy | 'inherit'
  ) => {
    if (mode === 'inherit') {
      setModelToolOverride(index, { responses_tool_parameters: undefined })
      return
    }
    const current =
      modelToolOverrides[index].responses_tool_parameters?.web_search
    setModelToolOverride(index, {
      responses_tool_parameters: {
        web_search:
          mode === 'populate_defaults'
            ? {
                when_nested_options_missing: mode,
                defaults: current?.defaults || {
                  enable: true,
                  search_result: true,
                },
              }
            : { when_nested_options_missing: mode },
      },
    })
  }
  const applyModelClientWebSearchFallback = (index: number) => {
    const override = modelToolOverrides[index]
    setModelToolOverride(index, {
      responses_tools: {
        ...override.responses_tools,
        web_search: 'drop',
      },
      responses_tool_parameters: undefined,
    })
  }
  const setModelWebSearchDefault = (
    index: number,
    key: 'enable' | 'search_result' | 'search_engine',
    value: boolean | string | undefined
  ) => {
    const current =
      modelToolOverrides[index].responses_tool_parameters?.web_search
    const defaults = updateWebSearchParameterDefaults(
      current?.defaults,
      key,
      value
    )
    setModelToolOverride(index, {
      responses_tool_parameters: {
        web_search: {
          when_nested_options_missing: 'populate_defaults',
          defaults,
        },
      },
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
        {
          models: [],
          responses_tools: {},
          responses_tool_names: [],
          responses_implicit_hosted_tools: [],
        },
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
    <div className='border-border flex flex-col gap-4 rounded-md border p-4 lg:gap-2 lg:p-3'>
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
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='lg:hidden'
            onClick={onRemove}
          >
            <Trash2 className='h-4 w-4' />
            <span className='sr-only'>{t('Delete')}</span>
          </Button>
        </div>

        <FieldBlock
          label={t('Incoming path')}
          className='lg:gap-1'
          labelClassName='lg:sr-only'
        >
          <Select value={incomingPath} onValueChange={setIncomingPath}>
            <SelectTrigger className='w-full max-w-full lg:h-8'>
              <SelectValue className='min-w-0 truncate'>
                {`${incomingPathLabel}`}
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
            placeholder={getAdvancedCustomUpstreamPathPlaceholder(converter)}
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
            onValueChange={(value) =>
              setConverter(value as AdvancedCustomConverter)
            }
          >
            <SelectTrigger className='w-full max-w-full lg:h-8'>
              <SelectValue className='min-w-0 truncate'>
                {t(converterLabel)}
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

        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='hidden lg:inline-flex'
          onClick={onRemove}
        >
          <Trash2 className='h-4 w-4' />
          <span className='sr-only'>{t('Delete')}</span>
        </Button>
      </div>

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
            <div className='grid gap-3 rounded border p-3 lg:grid-cols-2'>
              <div className='space-y-2'>
                <div>
                  <p className='text-xs font-medium'>
                    {t('Implicit hosted tools (route)')}
                  </p>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Declare hosted capabilities supplied by the upstream even when the client request omits them.'
                    )}
                  </p>
                </div>
                <div className='space-y-2'>
                  {ADVANCED_CUSTOM_IMPLICIT_HOSTED_TOOL_OPTIONS.map(
                    (option) => (
                      <label
                        key={option.value}
                        className='flex items-start gap-2 text-sm'
                      >
                        <Checkbox
                          checked={routeImplicitHostedTools.includes(
                            option.value
                          )}
                          onCheckedChange={(value) =>
                            setRouteImplicitHostedTool(
                              option.value,
                              value === true
                            )
                          }
                        />
                        <span className='min-w-0'>
                          <span className='block'>{t(option.label)}</span>
                          <span className='text-muted-foreground block text-xs'>
                            {t(option.description)}
                          </span>
                        </span>
                      </label>
                    )
                  )}
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t('Effective source')}:{' '}
                  {routeImplicitHostedTools.length > 0
                    ? t('route')
                    : t('system default (none)')}
                </p>
              </div>
              {!isNativeConverter ? (
                <div className='space-y-3 rounded-md border p-3'>
                  <div className='space-y-1'>
                    <p className='text-sm font-medium'>
                      {t('Web search schema compatibility (Responses to Chat)')}
                    </p>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Repairs the Chat request schema when a Responses web_search tool is converted without required nested parameters.'
                      )}
                    </p>
                  </div>
                  <Alert>
                    <AlertDescription className='space-y-2 text-xs'>
                      <p>
                        {t('Before compatibility')}:{' '}
                        <code>{'{"type":"web_search"}'}</code>
                      </p>
                      <p>
                        {t('After compatibility')}:{' '}
                        <code>
                          {
                            '{"type":"web_search","web_search":{"enable":true,"search_result":true}}'
                          }
                        </code>
                      </p>
                      <p>
                        {t(
                          'This transformation runs only when nested options are missing or empty. Explicit client values are never overwritten.'
                        )}
                      </p>
                      <p className='font-medium'>
                        {t(
                          'Schema compatibility does not provide or verify actual web search capability.'
                        )}
                      </p>
                      <p>
                        {t(
                          'For GLM CodingPlan, drop the hosted web_search tool and use a client-side search function such as the SearXNG MCP.'
                        )}
                      </p>
                    </AlertDescription>
                  </Alert>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={applyRouteClientWebSearchFallback}
                    >
                      {t('Use client-side search fallback')}
                    </Button>
                    <span className='text-muted-foreground text-xs'>
                      {t('Current status')}: {t(routeWebSearchStatus)}
                    </span>
                  </div>
                  <FieldBlock
                    label={t('Compatibility action')}
                    labelClassName='text-xs'
                  >
                    <Select
                      value={
                        routeWebSearchParameters?.when_nested_options_missing ||
                        'preserve'
                      }
                      onValueChange={(value) =>
                        setRouteWebSearchMode(
                          value as AdvancedCustomResponsesToolMissingOptionsPolicy
                        )
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='preserve'>
                          {t('Disabled: pass through unchanged')}
                        </SelectItem>
                        <SelectItem value='populate_defaults'>
                          {t('Enabled: fill missing parameters')}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  </FieldBlock>
                  {routeWebSearchParameters?.when_nested_options_missing ===
                  'populate_defaults' ? (
                    <div className='grid gap-2 sm:grid-cols-2'>
                      {(['enable', 'search_result'] as const).map((key) => (
                        <FieldBlock
                          key={key}
                          label={t(
                            key === 'enable'
                              ? 'Enable web search (enable)'
                              : 'Return search results (search_result)'
                          )}
                          labelClassName='text-xs'
                        >
                          <Select
                            value={
                              typeof routeWebSearchParameters.defaults?.[
                                key
                              ] === 'boolean'
                                ? String(
                                    routeWebSearchParameters.defaults?.[key]
                                  )
                                : 'unset'
                            }
                            onValueChange={(value) =>
                              setRouteWebSearchDefault(
                                key,
                                value === 'unset' ? undefined : value === 'true'
                              )
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='unset'>
                                {t('Unset')}
                              </SelectItem>
                              <SelectItem value='true'>true</SelectItem>
                              <SelectItem value='false'>false</SelectItem>
                            </SelectContent>
                          </Select>
                        </FieldBlock>
                      ))}
                      <FieldBlock
                        label={t('Search engine (optional)')}
                        labelClassName='text-xs sm:col-span-2'
                      >
                        <Input
                          value={
                            routeWebSearchParameters.defaults?.search_engine ||
                            ''
                          }
                          onChange={(event) =>
                            setRouteWebSearchDefault(
                              'search_engine',
                              event.target.value
                            )
                          }
                          placeholder={t(
                            'Leave empty to use the upstream default'
                          )}
                        />
                      </FieldBlock>
                    </div>
                  ) : null}
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Scope: this route and all matching models. Add a model override below when only specific models need schema compatibility or a client-side fallback.'
                    )}
                  </p>
                </div>
              ) : null}
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
                  <div className='bg-muted/30 grid gap-3 rounded p-2 lg:grid-cols-2'>
                    <div className='space-y-2'>
                      <p className='text-xs font-medium'>
                        {t('Implicit hosted tools (model)')}
                      </p>
                      {ADVANCED_CUSTOM_IMPLICIT_HOSTED_TOOL_OPTIONS.map(
                        (option) => (
                          <label
                            key={option.value}
                            className='flex items-center gap-2 text-sm'
                          >
                            <Checkbox
                              checked={(
                                override.responses_implicit_hosted_tools || []
                              ).includes(option.value)}
                              onCheckedChange={(value) =>
                                setModelImplicitHostedTool(
                                  overrideIndex,
                                  option.value,
                                  value === true
                                )
                              }
                            />
                            <span>{t(option.label)}</span>
                          </label>
                        )
                      )}
                      <p className='text-muted-foreground text-xs'>
                        {t('Effective source')}:{' '}
                        {t(
                          getImplicitHostedToolsSource(
                            routeImplicitHostedTools,
                            override.responses_implicit_hosted_tools || []
                          )
                        )}
                      </p>
                    </div>
                    {!isNativeConverter ? (
                      <div className='space-y-2 rounded-md border p-3'>
                        <div className='space-y-1'>
                          <p className='text-xs font-medium'>
                            {t(
                              'Web search schema compatibility (model override)'
                            )}
                          </p>
                          <p className='text-muted-foreground text-xs'>
                            {t(
                              'Use this override when only the selected models require schema compatibility or must drop hosted web_search in favor of a client-side function.'
                            )}
                          </p>
                        </div>
                        <div className='flex flex-wrap items-center gap-2'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() =>
                              applyModelClientWebSearchFallback(overrideIndex)
                            }
                          >
                            {t(
                              'Use client-side search fallback for these models'
                            )}
                          </Button>
                          <span className='text-muted-foreground text-xs'>
                            {t('Selected models')}:{' '}
                            {override.models?.length
                              ? override.models.join(', ')
                              : t('none')}
                          </span>
                        </div>
                        <FieldBlock
                          label={t('Compatibility action')}
                          labelClassName='text-xs'
                        >
                          <Select
                            value={
                              override.responses_tool_parameters?.web_search
                                ?.when_nested_options_missing || 'inherit'
                            }
                            onValueChange={(value) =>
                              setModelWebSearchMode(
                                overrideIndex,
                                value as
                                  | AdvancedCustomResponsesToolMissingOptionsPolicy
                                  | 'inherit'
                              )
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value='inherit'>
                                {t('Inherit route setting')}
                              </SelectItem>
                              <SelectItem value='preserve'>
                                {t('Disabled: pass through unchanged')}
                              </SelectItem>
                              <SelectItem value='populate_defaults'>
                                {t('Enabled: fill missing parameters')}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </FieldBlock>
                        {override.responses_tool_parameters?.web_search
                          ?.when_nested_options_missing ===
                        'populate_defaults' ? (
                          <div className='grid gap-2 sm:grid-cols-2'>
                            {(['enable', 'search_result'] as const).map(
                              (key) => (
                                <FieldBlock
                                  key={key}
                                  label={t(
                                    key === 'enable'
                                      ? 'Enable web search (enable)'
                                      : 'Return search results (search_result)'
                                  )}
                                  labelClassName='text-xs'
                                >
                                  <Select
                                    value={
                                      typeof override.responses_tool_parameters
                                        ?.web_search?.defaults?.[key] ===
                                      'boolean'
                                        ? String(
                                            override.responses_tool_parameters
                                              ?.web_search?.defaults?.[key]
                                          )
                                        : 'unset'
                                    }
                                    onValueChange={(value) =>
                                      setModelWebSearchDefault(
                                        overrideIndex,
                                        key,
                                        value === 'unset'
                                          ? undefined
                                          : value === 'true'
                                      )
                                    }
                                  >
                                    <SelectTrigger>
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                      <SelectItem value='unset'>
                                        {t('Unset')}
                                      </SelectItem>
                                      <SelectItem value='true'>true</SelectItem>
                                      <SelectItem value='false'>
                                        false
                                      </SelectItem>
                                    </SelectContent>
                                  </Select>
                                </FieldBlock>
                              )
                            )}
                            <FieldBlock
                              label={t('Search engine (optional)')}
                              labelClassName='text-xs sm:col-span-2'
                            >
                              <Input
                                value={
                                  override.responses_tool_parameters?.web_search
                                    ?.defaults?.search_engine || ''
                                }
                                onChange={(event) =>
                                  setModelWebSearchDefault(
                                    overrideIndex,
                                    'search_engine',
                                    event.target.value
                                  )
                                }
                                placeholder={t(
                                  'Leave empty to use the upstream default'
                                )}
                              />
                            </FieldBlock>
                          </div>
                        ) : null}
                        <p className='text-muted-foreground text-xs'>
                          {t(
                            'Only enable parameter filling after verifying that the upstream performs real web search. Filling fields alone only repairs the request schema.'
                          )}
                        </p>
                        <p className='text-muted-foreground text-xs'>
                          {t(
                            'The model override takes priority over the route setting, but explicit client values still take priority over both.'
                          )}
                        </p>
                      </div>
                    ) : null}
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

function FieldBlock({
  label,
  description,
  className,
  labelClassName,
  children,
}: {
  label: string
  description?: string
  className?: string
  labelClassName?: string
  children: ReactNode
}) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-2', className)}>
      <span className={cn('text-sm font-medium', labelClassName)} title={label}>
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
