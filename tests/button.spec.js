const { test, expect } = require('@playwright/test');

test.describe('Button demo', () => {
  test('increments the click counter', async ({ page }) => {
    await page.goto('/components/button');

    const counter = page.getByRole('button', { name: /^Clicked \d+ times$/ });

    await expect(counter).toHaveText('Clicked 0 times');

    await counter.click();

    await expect(counter).toHaveText('Clicked 1 times');
  });
});
