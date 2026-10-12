import type { Container } from '~/types/container';

export const containers: Container[] = [
  {
    id: '20-standard',
    title: '20 футов Standard',
    size: '20′',
    condition: 'Состояние уточняется',
    description: 'Компактный формат для хранения и перевозки.',
    features: ['Стандартная высота', 'Типоразмер 20 футов', 'Характеристики уточняются'],
    image: {
      src: '/images/container-20.svg',
      alt: 'Демонстрационная иллюстрация 20-футового контейнера',
      width: 800,
      height: 540,
    },
    price: null,
    demo: true,
  },
  {
    id: '40-standard',
    title: '40 футов Standard',
    size: '40′',
    condition: 'Состояние уточняется',
    description: 'Больше пространства для объёмных задач.',
    features: ['Стандартная высота', 'Типоразмер 40 футов', 'Характеристики уточняются'],
    image: {
      src: '/images/container-40.svg',
      alt: 'Демонстрационная иллюстрация 40-футового контейнера',
      width: 800,
      height: 540,
    },
    price: null,
    demo: true,
  },
  {
    id: '40-high-cube',
    title: '40 футов High Cube',
    size: '40′ HC',
    condition: 'Состояние уточняется',
    description: 'Увеличенная высота для нестандартных грузов.',
    features: ['Увеличенная высота', 'Типоразмер 40 футов', 'Характеристики уточняются'],
    image: {
      src: '/images/container-hc.svg',
      alt: 'Демонстрационная иллюстрация контейнера High Cube',
      width: 800,
      height: 540,
    },
    price: null,
    demo: true,
  },
];
