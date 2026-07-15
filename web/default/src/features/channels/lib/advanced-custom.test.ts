import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { AdvancedCustomConfig } from '../types'
import {
  mergeAdvancedCustomRouteCompatibilityConfig,
  normalizeAdvancedCustomResponsesToolType,
  parseAdvancedCustomConfig,
  resolveAdvancedCustomResponsesToolPolicy,
  responsesToolsFromMode,
  stringifyAdvancedCustomConfig,
  validateAdvancedCustomConfig,
} from './advanced-custom'

const validResponsesRoute = {
  incoming_path: '/v1/responses',
  upstream_path: '/v1/chat/completions',
  converter: 'openai_responses_to_openai_chat_completions' as const,
}

describe('advanced custom config round trip', () => {
  test('preserves compatibility policies and unknown future fields', () => {
    const raw: AdvancedCustomConfig = {
      future_config: { enabled: true },
      model_fetch_urls: [' /v1/models ', '/v1/models'],
      advanced_routes: [
        {
          ...validResponsesRoute,
          future_route: 'route-value',
          auth: {
            type: 'header',
            name: 'x-api-key',
            value: '{api_key}',
            future_auth: 1,
          },
          converter_options: {
            future_converter: { version: 2 },
            responses_tool_conflict_policy: 'deduplicate',
            responses_tools: {
              namespace: 'flatten',
              web_search: 'preserve',
              future_tool_policy: 'future-value',
            },
            responses_tool_model_overrides: [
              {
                future_override: true,
                models: [' glm-5.2 ', 'glm-5.2'],
                responses_tools: {
                  image_generation: 'drop',
                  future_model_tool: 'preserve-later',
                },
                responses_tool_names: [
                  {
                    tool_type: ' web_search_preview ',
                    tool_name: ' search_preview ',
                    policy: 'reject',
                    future_name_policy: 3,
                  },
                ],
              },
            ],
          },
        },
      ],
    }

    const parsed = parseAdvancedCustomConfig(JSON.stringify(raw))
    assert.ok(parsed)
    const roundTripped = JSON.parse(
      stringifyAdvancedCustomConfig(parsed)
    ) as AdvancedCustomConfig
    const route = roundTripped.advanced_routes?.[0]
    const options = route?.converter_options
    const override = options?.responses_tool_model_overrides?.[0]
    const namePolicy = override?.responses_tool_names?.[0]

    assert.deepEqual(roundTripped.future_config, { enabled: true })
    assert.deepEqual(roundTripped.model_fetch_urls, ['/v1/models'])
    assert.equal(route?.future_route, 'route-value')
    assert.equal(route?.auth?.future_auth, 1)
    assert.deepEqual(options?.future_converter, { version: 2 })
    assert.equal(options?.responses_tool_conflict_policy, 'deduplicate')
    assert.equal(options?.responses_tools?.future_tool_policy, 'future-value')
    assert.equal(override?.future_override, true)
    assert.deepEqual(override?.models, ['glm-5.2'])
    assert.equal(override?.responses_tools?.future_model_tool, 'preserve-later')
    assert.equal(namePolicy?.tool_type, 'web_search_preview')
    assert.equal(namePolicy?.tool_name, 'search_preview')
    assert.equal(namePolicy?.future_name_policy, 3)
  })
})

describe('advanced custom compatibility mutation merge', () => {
  test('updates only compatibility policy fields for the matching route', () => {
    const current: AdvancedCustomConfig = {
      future_config: { unsaved: true },
      model_fetch_urls: ['/v1/models-unsaved'],
      advanced_routes: [
        {
          ...validResponsesRoute,
          upstream_path: '/v1/chat/completions-unsaved',
          auth: { type: 'header', name: 'x-unsaved', value: 'local' },
          future_route: 'keep-local',
          converter_options: {
            future_converter: { keep: true },
            responses_drop_fields: ['metadata', 'local_field'],
            responses_tools: { namespace: 'flatten', web_search: 'drop' },
            responses_tool_model_overrides: [],
          },
        },
        {
          incoming_path: '/v1/chat/completions',
          upstream_path: '/unchanged',
          converter: 'none',
        },
      ],
    }

    const merged = mergeAdvancedCustomRouteCompatibilityConfig(current, {
      ...validResponsesRoute,
      upstream_path: '/persisted-should-not-overwrite-local',
      converter_options: {
        responses_tool_conflict_policy: 'deduplicate',
        responses_tools: { namespace: 'flatten' },
        responses_tool_model_overrides: [
          {
            models: ['glm-5.2'],
            responses_tools: { image_generation: 'drop' },
          },
        ],
        responses_drop_fields: ['persisted-should-not-overwrite-local'],
      },
    })

    assert.deepEqual(merged.future_config, { unsaved: true })
    assert.deepEqual(merged.model_fetch_urls, ['/v1/models-unsaved'])
    assert.equal(
      merged.advanced_routes?.[0]?.upstream_path,
      '/v1/chat/completions-unsaved'
    )
    assert.deepEqual(merged.advanced_routes?.[0]?.auth, {
      type: 'header',
      name: 'x-unsaved',
      value: 'local',
    })
    assert.equal(merged.advanced_routes?.[0]?.future_route, 'keep-local')
    assert.deepEqual(
      merged.advanced_routes?.[0]?.converter_options?.future_converter,
      { keep: true }
    )
    assert.deepEqual(
      merged.advanced_routes?.[0]?.converter_options?.responses_drop_fields,
      ['metadata', 'local_field']
    )
    const mergedTools =
      merged.advanced_routes?.[0]?.converter_options?.responses_tools
    assert.equal(mergedTools?.namespace, 'flatten')
    assert.equal(mergedTools?.web_search, undefined)
    assert.equal(mergedTools?.image_generation, undefined)
    assert.equal(
      merged.advanced_routes?.[0]?.converter_options
        ?.responses_tool_conflict_policy,
      'deduplicate'
    )
    const mergedOverrides =
      merged.advanced_routes?.[0]?.converter_options
        ?.responses_tool_model_overrides
    assert.equal(mergedOverrides?.length, 1)
    assert.deepEqual(mergedOverrides?.[0]?.models, ['glm-5.2'])
    assert.equal(
      mergedOverrides?.[0]?.responses_tools?.image_generation,
      'drop'
    )
    assert.equal(merged.advanced_routes?.[1]?.upstream_path, '/unchanged')
  })
})

describe('advanced custom tool policy resolution', () => {
  test('normalizes hosted tool aliases', () => {
    assert.equal(
      normalizeAdvancedCustomResponsesToolType('web_search_preview'),
      'web_search'
    )
    assert.equal(
      normalizeAdvancedCustomResponsesToolType('image_gen'),
      'image_generation'
    )
    assert.equal(
      normalizeAdvancedCustomResponsesToolType('future_client_tool'),
      'unknown'
    )
  })

  test('resolves protected, model name, model type, route and default sources', () => {
    const route = {
      ...validResponsesRoute,
      converter_options: {
        responses_tools: { namespace: 'flatten' as const },
        responses_tool_model_overrides: [
          {
            models: ['glm-5.2'],
            responses_tools: { image_generation: 'drop' as const },
            responses_tool_names: [
              {
                tool_type: 'web_search',
                tool_name: 'web_search_preview',
                policy: 'reject' as const,
              },
            ],
          },
        ],
      },
    }

    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(route, 'glm-5.2', 'function'),
      { policy: 'preserve', source: 'protected' }
    )
    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(
        route,
        'glm-5.2',
        'web_search_preview',
        'web_search_preview'
      ),
      {
        policy: 'reject',
        source: 'model_tool_name',
        matchedModel: 'glm-5.2',
      }
    )
    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(route, 'glm-5.2', 'image_gen'),
      {
        policy: 'drop',
        source: 'model_tool_type',
        matchedModel: 'glm-5.2',
      }
    )
    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(route, 'other', 'namespace'),
      { policy: 'flatten', source: 'route' }
    )
    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(route, 'other', 'web_search'),
      { policy: 'preserve', source: 'system_default' }
    )
  })

  test('prefers requested model override before mapped upstream model', () => {
    const route = {
      ...validResponsesRoute,
      converter_options: {
        responses_tool_model_overrides: [
          {
            models: ['glm-5.2'],
            responses_tools: { image_generation: 'drop' as const },
          },
          {
            models: ['alias-model'],
            responses_tools: { image_generation: 'reject' as const },
          },
        ],
      },
    }

    assert.deepEqual(
      resolveAdvancedCustomResponsesToolPolicy(
        route,
        'alias-model',
        'image_gen',
        '',
        'glm-5.2'
      ),
      {
        policy: 'reject',
        source: 'model_tool_type',
        matchedModel: 'alias-model',
      }
    )
  })

  test('expands legacy mode without disabling web search in preserve mode', () => {
    assert.equal(responsesToolsFromMode('preserve').web_search, 'preserve')
    assert.equal(responsesToolsFromMode('compat_flatten').namespace, 'flatten')
  })
})

describe('advanced custom tool policy validation', () => {
  test('rejects a model override that duplicates the route policy', () => {
    const error = validateAdvancedCustomConfig({
      advanced_routes: [
        {
          ...validResponsesRoute,
          converter_options: {
            responses_tools: { web_search: 'preserve' },
            responses_tool_model_overrides: [
              {
                models: ['glm-5.2'],
                responses_tools: { web_search: 'preserve' },
              },
            ],
          },
        },
      ],
    })

    assert.equal(error?.routeIndex, 0)
    assert.match(error?.message || '', /must differ from the route policy/)
  })

  test('ignores unknown future policy fields while validating known fields', () => {
    const error = validateAdvancedCustomConfig({
      advanced_routes: [
        {
          ...validResponsesRoute,
          converter_options: {
            responses_tools: {
              namespace: 'flatten',
              future_tool_policy: { behavior: 'future' },
            },
          },
        },
      ],
    })

    assert.equal(error, null)
  })
})
