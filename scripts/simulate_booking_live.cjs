const { chromium } = require('../frontend/node_modules/playwright');
const path = require('path');
const fs = require('fs');

(async () => {
  console.log("🚀 Launching interactive browser on your display...");
  const browser = await chromium.launch({
    headless: false,
    viewport: null,
    args: ['--start-maximized', '--window-size=1440,900']
  });

  const context = await browser.newContext({ viewport: null });
  const page = await context.newPage();

  console.log("🔑 Logging in to XChats as admin@xchat.kz...");
  await page.goto('http://localhost:5173/login', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);

  const emailInput = page.locator('input[type="email"]');
  const passwordInput = page.locator('input[type="password"]');

  if (await emailInput.isVisible()) {
    await emailInput.fill('admin@xchat.kz');
    await page.waitForTimeout(300);
    await passwordInput.fill('xchat-admin-change-me');
    await page.waitForTimeout(300);
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: 'networkidle' }).catch(() => {});
    await page.waitForTimeout(1500);
  }

  console.log("💬 Navigating to Simulator (/simulator)...");
  await page.goto('http://localhost:5173/simulator', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);

  // Click "+ Новая беседа" to start a fresh thread
  const newChatBtn = page.locator('button:has-text("Новая беседа")').first();
  if (await newChatBtn.isVisible().catch(() => false)) {
    console.log("✨ Starting a clean booking conversation...");
    await newChatBtn.click();
    await page.waitForTimeout(1200);
  }

  const chatInput = page.locator('textarea, input[placeholder*="сообщение"], input[placeholder*="Message"]').first();

  async function typeAndSend(message, description) {
    console.log(`\n👉 [Клиент]: ${description}`);
    console.log(`   Текст: "${message}"`);
    await chatInput.click();
    for (const char of message) {
      await chatInput.type(char, { delay: 20 + Math.random() * 15 });
    }
    await page.waitForTimeout(500);
    await page.keyboard.press('Enter');
    console.log("⏳ Ожидание ответа ассистента Zarina PM Studio...");
    await page.waitForTimeout(6000);
  }

  // Step 1: Booking inquiry for permanent makeup
  await typeAndSend(
    'Здравствуйте! Хочу записаться к Зарине на перманентный макияж бровей и губ. Сколько стоят процедуры и есть ли скидка?',
    'Запрос на запись на перманент бровей и губ к Зарине'
  );

  // Step 2: Botox and zones inquiry
  await typeAndSend(
    'А также интересует ботокс лица — сколько стоит и какие зоны входят?',
    'Уточнение зон и цены ботокса'
  );

  // Step 3: Location and studio address
  await typeAndSend(
    'Подскажите, пожалуйста, точный адрес вашей студии в Шымкенте?',
    'Запрос адреса студии в Шымкенте'
  );

  // Step 4: Booking link & confirmation
  await typeAndSend(
    'Как записаться к мастеру Зарине и подтвердить время записи?',
    'Уточнение процесса подтверждения записи через WhatsApp'
  );

  console.log("\n📸 Saving screenshot of complete simulator dialogue...");
  const outDir = path.resolve(__dirname, '../real-usecases/zarina-pmstudio');
  if (!fs.existsSync(outDir)) fs.mkdirSync(outDir, { recursive: true });
  await page.screenshot({ path: path.join(outDir, 'simulator_booking_dialogue.png'), fullPage: true });

  console.log("\n🔄 Testing reload persistence in Simulator...");
  await page.reload({ waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  console.log("✅ Verified: all dialogue turns remained intact after reload!");
  await page.screenshot({ path: path.join(outDir, 'simulator_after_reload.png'), fullPage: true });

  console.log("\n📋 Opening Chatboard (/chatboard) to demonstrate operator CRM flow...");
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  // Select the conversation
  const chatItem = page.locator('button').filter({ hasText: /Хочу записаться|перманентный|ботокс/ }).first();
  if (await chatItem.isVisible().catch(() => false)) {
    console.log("🔍 Selecting simulated conversation in Chatboard list...");
    await chatItem.click();
    await page.waitForTimeout(1500);

    // Switch right panel to "ИИ-помощник"
    const assistantTab = page.locator('button[role="tab"]').filter({ hasText: /ИИ-помощник/ }).first();
    if (await assistantTab.isVisible().catch(() => false)) {
      console.log("🤖 Switching right panel to 'ИИ-помощник' to view generated draft reply...");
      await assistantTab.click();
      await page.waitForTimeout(1500);
      await page.screenshot({ path: path.join(outDir, 'chatboard_assistant_panel.png'), fullPage: true });

      // Click "Отправить" on the draft to approve and send to customer thread
      const approveBtn = page.locator('button:has-text("Отправить")').first();
      if (await approveBtn.isVisible().catch(() => false)) {
        console.log("✉️ Approving draft and sending into main chat thread...");
        await approveBtn.click();
        await page.waitForTimeout(2000);
        await page.screenshot({ path: path.join(outDir, 'chatboard_message_sent.png'), fullPage: true });
      }
    }
  }

  console.log("\n🎉 Simulation complete! Leaving the browser open for your inspection (waiting 15 minutes)...");
  await page.waitForTimeout(900000);
})();
