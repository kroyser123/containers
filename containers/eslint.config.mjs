import withNuxt from './.nuxt/eslint.config.mjs';

export default withNuxt({
  ignores: ['.output/**', 'playwright-report/**', 'test-results/**'],
});
