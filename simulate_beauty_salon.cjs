const { chromium } = require('./frontend/node_modules/playwright');

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

(async () => {
  console.log('🚀 Launching Chrome browser on display :0...');
  const browser = await chromium.launch({
    headless: false,
    executablePath: '/usr/bin/google-chrome',
    args: ['--no-sandbox', '--start-maximized', '--window-size=1440,900']
  });

  const context = await browser.newContext({
    viewport: null
  });
  const page = await context.newPage();

  console.log('📍 Navigating to Login page (http://localhost:5173/login)...');
  await page.goto('http://localhost:5173/login', { waitUntil: 'networkidle' });
  await sleep(1500);

  console.log('🔑 Logging in as admin@xchat.kz...');
  await page.fill('input[type="email"]', 'admin@xchat.kz');
  await sleep(500);
  await page.fill('input[type="password"]', 'xchat-admin-change-me');
  await sleep(500);
  await page.click('button[type="submit"]');

  await page.waitForNavigation({ waitUntil: 'networkidle' }).catch(() => {});
  await sleep(1500);

  console.log('📖 Opening Knowledge Base (/knowledge-base)...');
  await page.goto('http://localhost:5173/knowledge-base', { waitUntil: 'networkidle' });
  await sleep(2000);

  // 1. Specialists Tab
  console.log('💇 Viewing Specialists (Специалисты)...');
  const specialistsTab = page.locator('button', { hasText: 'Специалисты' });
  if (await specialistsTab.count() > 0) {
    await specialistsTab.first().click();
    await sleep(2500);
    // Smooth scroll down and up to show all specialists
    await page.evaluate(() => window.scrollBy({ top: 300, behavior: 'smooth' }));
    await sleep(2000);
    await page.evaluate(() => window.scrollBy({ top: -300, behavior: 'smooth' }));
    await sleep(1000);
  }

  // 2. Services Tab
  console.log('✂️ Viewing Services (Услуги)...');
  const servicesTab = page.locator('button', { hasText: 'Услуги' });
  if (await servicesTab.count() > 0) {
    await servicesTab.first().click();
    await sleep(2500);
    // Scroll through services
    await page.evaluate(() => window.scrollBy({ top: 400, behavior: 'smooth' }));
    await sleep(2000);
    await page.evaluate(() => window.scrollBy({ top: 400, behavior: 'smooth' }));
    await sleep(2000);
    await page.evaluate(() => window.scrollBy({ top: -800, behavior: 'smooth' }));
    await sleep(1000);
  }

  // 3. Contacts Tab
  console.log('📍 Viewing Contacts (Контакты)...');
  const contactsTab = page.locator('button', { hasText: 'Контакты' });
  if (await contactsTab.count() > 0) {
    await contactsTab.first().click();
    await sleep(2500);
  }

  // 4. Simulator Page
  console.log('💬 Navigating to Simulator (/simulator)...');
  await page.goto('http://localhost:5173/simulator', { waitUntil: 'networkidle' });
  await sleep(2000);

  console.log('💬 Simulating beauty salon customer message 1...');
  const inputEl = page.locator('[data-testid="simulator-input"]');
  await inputEl.fill('Здравствуйте! Подскажите, какие стрижки у вас есть и сколько стоит женская стрижка?');
  await sleep(1000);
  await page.locator('[data-testid="simulator-send"]').click();

  // Wait for reply
  console.log('⏳ Waiting for assistant response...');
  await page.waitForSelector('[data-testid="simulator-message"][data-role="assistant"]', { timeout: 15000 }).catch(() => {});
  await sleep(3500);

  console.log('💬 Simulating beauty salon customer message 2...');
  await inputEl.fill('А кто делает маникюр и как записаться к Диане Нур?');
  await sleep(1000);
  await page.locator('[data-testid="simulator-send"]').click();
  await sleep(4000);

  // 5. Chatboard (Inbox)
  console.log('📥 Navigating to Customer Inbox (/chatboard)...');
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await sleep(3000);

  // 6. Return to Simulator so user has it ready
  console.log('✨ Returning to Simulator panel for interactive testing...');
  await page.goto('http://localhost:5173/simulator', { waitUntil: 'networkidle' });
  await sleep(1500);

  console.log('🎉 Tour completed! Leaving the browser open for the user.');
  // Keep process alive so browser remains open for the user
  await new Promise(() => {});
})().catch(err => {
  console.error('Simulation error:', err);
});
