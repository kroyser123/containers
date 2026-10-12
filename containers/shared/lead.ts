export interface Lead {
  name: string;
  phone: string;
  containerId: string;
  comment: string;
  consent: true;
  website: string;
}

export type LeadErrors = Partial<Record<keyof Lead, string>>;
export type ValidationResult = { data: Lead; errors?: never } | { errors: LeadErrors; data?: never };

export function validateLead(input: unknown, allowedIds: readonly string[]): ValidationResult {
  if (!input || typeof input !== 'object' || Array.isArray(input)) {
    return { errors: { name: 'Проверьте поля формы.' } };
  }
  const source = input as Record<string, unknown>;
  const keys = ['name', 'phone', 'containerId', 'comment', 'consent', 'website'];
  if (Object.keys(source).some((key) => !keys.includes(key))) {
    return { errors: { name: 'Запрос содержит неизвестные поля.' } };
  }
  const text = (key: string) => (typeof source[key] === 'string' ? source[key].trim() : '');
  const name = text('name');
  const phone = text('phone');
  const containerId = text('containerId');
  const comment = text('comment');
  const website = text('website');
  const errors: LeadErrors = {};
  if (name.length < 2 || name.length > 80 || /[\u0000-\u001f<>]/.test(name)) {
    errors.name = 'Введите имя от 2 до 80 символов.';
  }
  const digits = phone.replace(/\D/g, '');
  if (!/^\+?[\d\s()-]+$/.test(phone) || digits.length < 10 || digits.length > 15 || phone.length > 32) {
    errors.phone = 'Введите телефон с кодом страны: от 10 до 15 цифр.';
  }
  if ((source.containerId !== undefined && typeof source.containerId !== 'string') ||
      (containerId && !allowedIds.includes(containerId))) {
    errors.containerId = 'Выберите контейнер из списка.';
  }
  if ((source.comment !== undefined && typeof source.comment !== 'string') || comment.length > 1000) {
    errors.comment = 'Комментарий — не более 1000 символов.';
  }
  if (source.consent !== true) errors.consent = 'Необходимо согласие на обработку данных.';
  if ((source.website !== undefined && typeof source.website !== 'string') || website) {
    errors.website = 'Не удалось отправить форму.';
  }
  if (Object.keys(errors).length) return { errors };
  return { data: { name, phone: `+${digits}`, containerId, comment, consent: true, website: '' } };
}
