export function formatPrice(price: number | null): string {
  return price === null ? 'Цена по запросу' : new Intl.NumberFormat('ru-RU', {
    style: 'currency', currency: 'RUB', maximumFractionDigits: 0,
  }).format(price);
}
