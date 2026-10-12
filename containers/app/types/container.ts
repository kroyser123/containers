export interface Container {
  id: string;
  title: string;
  size: string;
  condition: string;
  description: string;
  features: string[];
  image: { src: string; alt: string; width: number; height: number; srcset?: string };
  price: number | null;
  demo: boolean;
}
