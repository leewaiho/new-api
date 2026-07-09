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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Edit, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { ensureVolcengineCodingPlanCatalog, getVendors } from '../../api'
import { handleDeleteVendor } from '../../lib/vendor-actions'
import { modelsQueryKeys, vendorsQueryKeys } from '../../lib/query-keys'
import { useModels } from '../models-provider'

type VendorManagementDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VendorManagementDialog({
  open,
  onOpenChange,
}: VendorManagementDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { setOpen, setCurrentVendor } = useModels()

  const vendorsQuery = useQuery({
    queryKey: vendorsQueryKeys.list({ page_size: 1000 }),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: open,
  })

  const vendors = vendorsQuery.data?.data?.items || []

  const handleCreate = () => {
    setCurrentVendor(null)
    setOpen('create-vendor')
  }

  const handleEdit = (vendor: (typeof vendors)[number]) => {
    setCurrentVendor(vendor)
    setOpen('update-vendor')
  }

  const handleDelete = async (id: number, modelCount?: number) => {
    if (
      !window.confirm(
        modelCount && modelCount > 0
          ? t('This vendor has linked models. Delete anyway?')
          : t('Delete this vendor?')
      )
    ) {
      return
    }
    await handleDeleteVendor(id, queryClient)
  }

  const handleSeedVolcengine = async () => {
    try {
      const response = await ensureVolcengineCodingPlanCatalog()
      if (!response.success) {
        toast.error(response.message || t('Failed to initialize catalog'))
        return
      }
      toast.success(t('Volcengine CodingPlan catalog initialized'))
      queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() })
      queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to initialize catalog')
      )
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Manage Vendors')}
      description={t('Create, edit, delete and initialize model vendors.')}
      contentClassName='sm:max-w-4xl'
      footer={
        <div className='flex w-full flex-wrap justify-between gap-2'>
          <Button type='button' variant='outline' onClick={handleSeedVolcengine}>
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('Initialize Volcengine CodingPlan')}
          </Button>
          <Button type='button' onClick={handleCreate}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add Vendor')}
          </Button>
        </div>
      }
    >
      {vendorsQuery.isLoading ? (
        <div className='flex items-center justify-center py-10 text-sm text-muted-foreground'>
          <Loader2 className='mr-2 h-4 w-4 animate-spin' />
          {t('Loading')}
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Name')}</TableHead>
              <TableHead>{t('Icon')}</TableHead>
              <TableHead>{t('Models')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {vendors.map((vendor) => (
              <TableRow key={vendor.id}>
                <TableCell>
                  <div className='font-medium'>{vendor.name}</div>
                  {vendor.description ? (
                    <div className='text-muted-foreground max-w-md truncate text-xs'>
                      {vendor.description}
                    </div>
                  ) : null}
                </TableCell>
                <TableCell>{vendor.icon || '-'}</TableCell>
                <TableCell>{vendor.model_count || 0}</TableCell>
                <TableCell>
                  <Badge variant={vendor.status === 1 ? 'default' : 'secondary'}>
                    {vendor.status === 1 ? t('Enabled') : t('Disabled')}
                  </Badge>
                </TableCell>
                <TableCell>
                  <div className='flex justify-end gap-2'>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => handleEdit(vendor)}
                    >
                      <Edit className='h-4 w-4' />
                    </Button>
                    <Button
                      type='button'
                      variant='destructive'
                      size='sm'
                      onClick={() => handleDelete(vendor.id, vendor.model_count)}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {vendors.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className='text-muted-foreground py-10 text-center'>
                  {t('No vendors found')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      )}
    </Dialog>
  )
}
