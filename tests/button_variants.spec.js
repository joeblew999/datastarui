const { test, expect } = require('@playwright/test');

const VARIANTS = ['Default', 'Destructive', 'Outline', 'Secondary', 'Ghost', 'Link'];

test.describe('Button variants demo', () => {
  test('renders all documented variant buttons', async ({ page }) => {
    await page.goto('/components/button');

    const variantsHeading = page.getByRole('heading', { level: 3, name: 'Variants' });
    await expect(variantsHeading).toBeVisible();

    const variantsContainer = variantsHeading.locator('..');
    for (const variant of VARIANTS) {
      await expect(variantsContainer.getByRole('button', { name: variant })).toBeVisible();
    }
  });
});
