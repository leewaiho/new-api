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
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getVendors } from '@/features/models/api'
import { vendorsQueryKeys } from '@/features/models/lib/query-keys'

import {
  applyChannelVendorCatalogModels,
  previewChannelVendorCatalogModels,
} from '../../api'
import { channelsQueryKeys } from '../../lib'
import type { VendorCatalogImportMode } from '../../types'

type VendorModelImportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelId?: number
  onApplied: (models: string[]) => void
}

function ModelListPreview({
  title,
  models,
  variant = 'secondary',
}: {
  title: string
  models: string[]
  variant?: 'secondary' | 'default' | 'destructive'
}) {
  return (
    <div className='space-y-2'>
      <div className='text-sm font-medium'>
        {title} ({models.length})
      </div>
      <div className='flex max-h-32 flex-wrap gap-1 overflow-y-auto rounded-md border p-2'>
        {models.length ? (
          models.map((model) => (
            <Badge key={model} variant={variant}>
              {model}
            </Badge>
          ))
        ) : (
          <span className='text-muted-foreground text-sm'>-</span>
        )}
      </div>
    </div>
  )
}

export function VendorModelImportDialog({
  open,
  onOpenChange,
  channelId,
  onApplied,
}: VendorModelImportDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [vendorId, setVendorId] = useState<string>('')
  const [mode, setMode] = useState<VendorCatalogImportMode>('append')

  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: open,
  })

  const vendors = useMemo(
    () => vendorsQuery.data?.data?.items?.filter((vendor) => vendor.status === 1) || [],
    [vendorsQuery.data?.data?.items]
  )

  useEffect(() => {
    if (open && !vendorId && vendors.length > 0) {
      setVendorId(String(vendors[0].id))
    }
  }, [open, vendorId, vendors])

  const selectedVendorId = Number(vendorId || 0)
  const previewQuery = useQuery({
    queryKey: ['channel-vendor-catalog-preview', channelId, selectedVendorId, mode],
    queryFn: () => previewChannelVendorCatalogModels(channelId || 0, selectedVendorId, mode),
    enabled: open && Boolean(channelId) && selectedVendorId > 0,
  })

  const preview = previewQuery.data?.data

  const applyMutation = useMutation({
    mutationFn: () =>
      applyChannelVendorCatalogModels({
        channel_id: channelId || 0,
        vendor_id: selectedVendorId,
        mode,
      }),
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(response.message || t('Failed to import vendor models'))
        return
      }
      onApplied(response.data.next_models)
      toast.success(t('Vendor models imported'))
      queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      if (channelId) {
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.detail(channelId) })
      }
      onOpenChange(false)
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Failed to import vendor models'))
    },
  })

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Import from Vendor Catalog')}
      description={t('Import locally maintained vendor model catalog into this channel.')}
      contentClassName='sm:max-w-3xl'
      footer={
        <>
          <Button type='button' variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => applyMutation.mutate()}
            disabled={!preview || applyMutation.isPending}
          >
            {applyMutation.isPending ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
            {t('Apply Import')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='grid gap-4 sm:grid-cols-2'>
          <div className='space-y-2'>
            <Label>{t('Vendor')}</Label>
            <Select value={vendorId} onValueChange={(value) => setVendorId(value || '')}>
              <SelectTrigger>
                <SelectValue placeholder={t('Select vendor')} />
              </SelectTrigger>
              <SelectContent>
                {vendors.map((vendor) => (
                  <SelectItem key={vendor.id} value={String(vendor.id)}>
                    {vendor.name} ({vendor.model_count || 0})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-2'>
            <Label>{t('Import Mode')}</Label>
            <Select value={mode} onValueChange={(value) => setMode(value as VendorCatalogImportMode)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='append'>{t('Append missing models')}</SelectItem>
                <SelectItem value='replace'>{t('Replace with vendor catalog')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        {previewQuery.isLoading ? (
          <div className='text-muted-foreground flex items-center justify-center py-8 text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Loading')}
          </div>
        ) : preview ? (
          <div className='space-y-4'>
            <div className='grid gap-4 sm:grid-cols-3'>
              <ModelListPreview title={t('Add Models')} models={preview.add_models} variant='default' />
              <ModelListPreview title={t('Keep Models')} models={preview.keep_models} />
              <ModelListPreview title={t('Remove Models')} models={preview.remove_models} variant='destructive' />
            </div>
            <ModelListPreview title={t('Result Models')} models={preview.next_models} />
          </div>
        ) : (
          <div className='text-muted-foreground rounded-md border p-4 text-sm'>
            {channelId ? t('Select vendor to preview changes') : t('Save channel before importing vendor catalog')}
          </div>
        )}
      </div>
    </Dialog>
  )
}
