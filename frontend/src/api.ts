export interface TelegramIntegration {
  id: number
  shop_id: number
  bot_token: string
  chat_id: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface TelegramStatus {
  enabled: boolean
  chat_id: string
  last_sent_at: string | null
  sent_count: number
  failed_count: number
  bot_connected: boolean
}

export interface OrderResponse {
  order: {
    id: number
    shop_id: number
    number: string
    total: number
    customer_name: string
    created_at: string
  }
  send_status: 'sent' | 'failed' | 'skipped'
}

export interface SendLogEntry {
  order_id: number
  order_number: string
  total: number
  customer_name: string
  message: string
  status: string
  error: string
  sent_at: string
}

const API = (shopId: number) => `/shops/${shopId}`

async function parseError(res: Response): Promise<never> {
  const text = await res.text()
  try {
    const json = JSON.parse(text)
    throw new Error(json.error || text)
  } catch (e) {
    if (e instanceof SyntaxError) throw new Error(text)
    throw e
  }
}

export async function connectTelegram(shopId: number, data: {
  botToken: string
  chatId: string
  enabled: boolean
}): Promise<TelegramIntegration> {
  const res = await fetch(`${API(shopId)}/telegram/connect`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function getTelegramStatus(shopId: number): Promise<TelegramStatus> {
  const res = await fetch(`${API(shopId)}/telegram/status`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function getSendLog(shopId: number): Promise<SendLogEntry[]> {
  const res = await fetch(`${API(shopId)}/telegram/log`)
  if (!res.ok) await parseError(res)
  return res.json()
}

export async function createOrder(shopId: number, data: {
  number: string
  total: number
  customerName: string
}): Promise<OrderResponse> {
  const res = await fetch(`${API(shopId)}/orders`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  })
  if (!res.ok) await parseError(res)
  return res.json()
}
