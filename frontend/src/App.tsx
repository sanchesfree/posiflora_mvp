import { useState, useEffect, useCallback, useRef } from 'react'
import { toast } from 'sonner'
import { connectTelegram, getTelegramStatus, getSendLog, createOrder } from './api'
import type { TelegramStatus, SendLogEntry } from './api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

const SHOP_ID = 1

function App() {
  // Telegram config
  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [enabled, setEnabled] = useState(false)

  // Status
  const [status, setStatus] = useState<TelegramStatus | null>(null)
  const [loading, setLoading] = useState(true)

  // Settings panel
  const [showSettings, setShowSettings] = useState(false)
  const settingsRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (showSettings && settingsRef.current) {
      settingsRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [showSettings, settingsRef])
  const [saving, setSaving] = useState(false)

  // Order form
  const [orderNumber, setOrderNumber] = useState('')
  const [orderTotal, setOrderTotal] = useState('')
  const [orderCustomer, setOrderCustomer] = useState('')
  const [creating, setCreating] = useState(false)

  // Log
  const [log, setLog] = useState<SendLogEntry[]>([])

  const loadStatus = useCallback(async () => {
    try {
      const s = await getTelegramStatus(SHOP_ID)
      setStatus(s)
      setEnabled(s.enabled)
    } catch {
      // No integration yet — default enabled ON for first setup
      setEnabled(true)
    } finally {
      setLoading(false)
    }
  }, [])

  const loadLog = useCallback(async () => {
    try {
      const entries = await getSendLog(SHOP_ID)
      setLog(entries)
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => { loadStatus(); loadLog() }, [loadStatus, loadLog])

  const handleSave = async () => {
    setSaving(true)
    try {
      await connectTelegram(SHOP_ID, { botToken, chatId, enabled })
      toast.success('Telegram-бот сохранён')
      setShowSettings(false)
      loadStatus()
      loadLog()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Ошибка сохранения')
    } finally {
      setSaving(false)
    }
  }

  const handleCreateOrder = async () => {
    if (!orderNumber.trim()) { toast.warning('Укажите номер заказа'); return }
    if (!orderTotal || parseFloat(orderTotal) <= 0) { toast.warning('Укажите сумму заказа'); return }
    if (!orderCustomer.trim()) { toast.warning('Укажите имя клиента'); return }

    setCreating(true)
    try {
      const resp = await createOrder(SHOP_ID, {
        number: orderNumber,
        total: parseFloat(orderTotal),
        customerName: orderCustomer,
      })

      if (resp.send_status === 'sent') {
        toast.success(`Заказ ${resp.order.number} создан и отправлен в Telegram`)
      } else if (resp.send_status === 'failed') {
        toast.error(`Заказ ${resp.order.number} создан, но не отправлен в Telegram`)
      } else {
        toast.info(`Заказ ${resp.order.number} создан (Telegram отключён)`)
      }

      setOrderNumber('')
      setOrderTotal('')
      setOrderCustomer('')
      loadStatus()
      loadLog()
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : 'Ошибка создания заказа')
    } finally {
      setCreating(false)
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 flex items-center justify-center">
        <div className="text-muted-foreground text-lg">Загрузка...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100">
      {/* Header */}
      <header className="border-b bg-white/80 backdrop-blur-sm sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-lg bg-primary flex items-center justify-center">
              <svg className="h-5 w-5 text-primary-foreground" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
              </svg>
            </div>
            <div>
              <h1 className="text-lg font-semibold tracking-tight">Posiflora</h1>
              <p className="text-sm text-muted-foreground">Telegram-интеграция</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Badge variant={status?.enabled ? 'default' : 'secondary'} className="text-sm px-3 py-1">
              {status?.enabled ? '✓ Подключено' : 'Отключено'}
            </Badge>
            <Button variant="outline" size="sm" onClick={() => setShowSettings(!showSettings)}>
              ⚙ Настроить
            </Button>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-6 py-8 space-y-6">
        {/* Collapsible Settings */}
        {showSettings && (
          <div ref={settingsRef}>
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Настройки бота</CardTitle>
              <CardDescription className="text-sm">
                {status?.bot_connected
                  ? 'Бот подключён. Оставьте поля пустыми чтобы только переключить уведомления.'
                  : 'Укажите Bot Token и Chat ID для подключения.'}
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="botToken" className="text-sm">Bot Token</Label>
                  <Input
                    id="botToken"
                    type="password"
                    placeholder="123456:ABC-DEF..."
                    value={botToken}
                    onChange={e => setBotToken(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="chatId" className="text-sm">Chat ID</Label>
                  <Input
                    id="chatId"
                    placeholder="987654321"
                    value={chatId}
                    onChange={e => setChatId(e.target.value)}
                  />
                </div>
              </div>
              <div className="flex items-center justify-between">
                <Label htmlFor="enabled" className="text-sm">Включить уведомления</Label>
                <Switch id="enabled" checked={enabled} onCheckedChange={setEnabled} />
              </div>
              <Separator />
              <div className="rounded-lg bg-amber-50/80 border border-amber-200/50 p-3">
                <p className="text-sm font-medium mb-1.5">💡 Как узнать Chat ID</p>
                <ol className="text-xs text-muted-foreground space-y-0.5 list-decimal list-inside">
                  <li>Создайте бота через <a href="https://t.me/BotFather" target="_blank" rel="noreferrer" className="underline text-foreground">@BotFather</a></li>
                  <li>Напишите боту любое сообщение</li>
                  <li>Откройте: <code className="bg-muted px-1 py-0.5 rounded text-[11px] font-mono">https://api.telegram.org/bot{'{TOKEN}'}/getUpdates</code></li>
                  <li>Найдите <code className="bg-muted px-1 py-0.5 rounded text-[11px] font-mono">chat.id</code> в ответе</li>
                </ol>
              </div>
              <Button className="w-full" onClick={handleSave} disabled={saving}>
                {saving ? 'Сохранение...' : 'Сохранить настройки'}
              </Button>
            </CardContent>
          </Card>
          </div>
        )}

        {/* Status */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Статус интеграции</CardTitle>
            <CardDescription className="text-sm">Магазин #{SHOP_ID}</CardDescription>
          </CardHeader>
          <CardContent>
            {status ? (
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-5">
                <StatusItem label="Статус" value={status.enabled ? 'Активна' : 'Выключена'} active={status.enabled} />
                <StatusItem label="Chat ID" value={status.chat_id || '—'} />
                <StatusItem label="Отправлено" value={`${status.sent_count}`} hint="за 7 дней" />
                <StatusItem label="Ошибки" value={`${status.failed_count}`} hint="за 7 дней" active={status.failed_count > 0} danger />
                {status.last_sent_at && (
                  <div className="col-span-full text-sm text-muted-foreground pt-1">
                    Последняя отправка: {new Date(status.last_sent_at).toLocaleString('ru')}
                  </div>
                )}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">Интеграция не подключена. Нажмите «Настроить» в шапке.</p>
            )}
          </CardContent>
        </Card>

        {/* Create Order */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Новый заказ</CardTitle>
            <CardDescription className="text-sm">Создайте заказ — уведомление уйдёт в Telegram</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 sm:grid-cols-3">
              <div className="space-y-2">
                <Label htmlFor="orderNumber" className="text-sm">Номер заказа</Label>
                <Input
                  id="orderNumber"
                  placeholder="A-1010"
                  value={orderNumber}
                  onChange={e => setOrderNumber(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="orderTotal" className="text-sm">Сумма (₽)</Label>
                <Input
                  id="orderTotal"
                  type="number"
                  min="1"
                  step="0.01"
                  placeholder="1500"
                  value={orderTotal}
                  onChange={e => setOrderTotal(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="orderCustomer" className="text-sm">Имя клиента</Label>
                <Input
                  id="orderCustomer"
                  placeholder="Иван"
                  value={orderCustomer}
                  onChange={e => setOrderCustomer(e.target.value)}
                />
              </div>
            </div>
            <div className="mt-4">
              <Button onClick={handleCreateOrder} disabled={creating} className="text-sm">
                {creating ? 'Создание...' : 'Создать заказ'}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Send Log */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Лог отправок</CardTitle>
            <CardDescription className="text-sm">История уведомлений в Telegram</CardDescription>
          </CardHeader>
          <CardContent>
            {log.length === 0 ? (
              <p className="text-sm text-muted-foreground py-6 text-center">
                Пока нет записей. Создайте заказ, чтобы увидеть лог.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="pb-2 pr-4 font-medium">Заказ</th>
                      <th className="pb-2 pr-4 font-medium">Клиент</th>
                      <th className="pb-2 pr-4 font-medium text-right">Сумма</th>
                      <th className="pb-2 pr-4 font-medium">Статус</th>
                      <th className="pb-2 font-medium">Время</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y">
                    {log.map((entry) => (
                      <tr key={`${entry.order_id}-${entry.sent_at}`} className="text-sm">
                        <td className="py-2.5 pr-4 font-medium">{entry.order_number}</td>
                        <td className="py-2.5 pr-4 text-muted-foreground">{entry.customer_name}</td>
                        <td className="py-2.5 pr-4 text-right tabular-nums">{entry.total.toLocaleString('ru')} ₽</td>
                        <td className="py-2.5 pr-4">
                          <Badge
                            variant={entry.status === 'SENT' ? 'default' : entry.status === 'SKIPPED' ? 'secondary' : 'destructive'}
                            className="text-xs"
                          >
                            {entry.status === 'SENT' ? '✓ Отправлено' : entry.status === 'SKIPPED' ? '⊘ Пропущено' : '✕ Ошибка'}
                          </Badge>
                          {entry.error && (
                            <div className="text-xs text-destructive/80 mt-0.5 max-w-xs truncate" title={entry.error}>
                              {entry.error}
                            </div>
                          )}
                        </td>
                        <td className="py-2.5 text-muted-foreground text-xs whitespace-nowrap">
                          {new Date(entry.sent_at).toLocaleString('ru')}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

      </main>
    </div>
  )
}

function StatusItem({ label, value, hint, active, danger }: {
  label: string
  value: string
  hint?: string
  active?: boolean
  danger?: boolean
}) {
  return (
    <div className="space-y-0.5">
      <div className="text-xs text-muted-foreground">{label} {hint && <span className="opacity-60">({hint})</span>}</div>
      <div className={`text-base font-medium ${danger ? 'text-destructive' : active ? 'text-emerald-600' : ''}`}>
        {value}
      </div>
    </div>
  )
}

export default App
