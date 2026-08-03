import { chromium } from '@playwright/test';

(async () => {
  console.log("Starting browser...");
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();

  await page.goto('http://localhost:8081/login', { waitUntil: 'domcontentloaded' });
  await page.locator('input[type=email]').fill('admin@xchats.test');
  await page.locator('input[type=password]').fill('change-me-strong-password');
  await page.getByRole('button', { name: 'Войти' }).click();
  await page.waitForTimeout(3000);
  console.log("Current URL after login:", page.url());
  if (page.url().includes('login')) {
      console.log("Login failed! Text on page:", await page.locator('body').innerText());
  }

  console.log("--- KNOWLEDGE BASE ---");
  await page.goto('http://localhost:8081/knowledge-base', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000); 

  // Print all tab buttons
  const kbTabs = await page.locator('.flex-wrap button').allTextContents();
  console.log("KB Tabs:", kbTabs.map(t => t.trim()));

  // Click 'Темы' tab if it exists
  const tabs = page.locator('.flex-wrap button');
  const count = await tabs.count();
  for (let i = 0; i < count; i++) {
    const text = await tabs.nth(i).textContent();
    if (text.includes('Темы')) {
      await tabs.nth(i).click();
      await page.waitForTimeout(1000);
      console.log("Clicked Темы tab.");
      const inputs = await page.locator('input, textarea').count();
      const buttons = await page.locator('button').allTextContents();
      console.log("KB Темы inputs count:", inputs);
      console.log("KB Темы buttons:", buttons.map(t => t.trim()).filter(t => t.length > 0));
    }
    if (text.includes('Зоны доставки')) {
      await tabs.nth(i).click();
      await page.waitForTimeout(1000);
      console.log("Clicked Зоны доставки tab.");
      // check if it has 'Активен для продажи'
      const html = await page.content();
      console.log("KB Zones has 'Активен для продажи'?", html.includes('Активен для продажи'));
      // try to click "+ Зона доставки" button (or similar toolbar button)
      const toolbarButtons = await page.locator('.border-dashed').allTextContents();
      console.log("KB Zones toolbar buttons:", toolbarButtons.map(t => t.trim()));
    }
  }

  console.log("--- PLAYGROUND ---");
  await page.goto('http://localhost:8081/playground', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000); 
  
  const pgTabs = await page.locator('.flex-wrap button').allTextContents();
  console.log("Draft Tabs:", pgTabs.map(t => t.trim()));
  
  const draftsHtml = await page.content();
  console.log("Draft inputs count:", await page.locator('input, textarea').count());
  console.log("Draft has 'Было:' text?", draftsHtml.includes('Было:'));

  await browser.close();
})().catch(err => {
  console.error("Error:", err);
  process.exit(1);
});
