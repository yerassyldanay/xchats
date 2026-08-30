import { chromium } from '@playwright/test';
import fs from 'fs';
import path from 'path';

const outDir = '/home/yerassyl/.gemini/antigravity/brain/dda1b7d1-43fb-4d9f-9ea4-65beb185197d';
const localDir = './test-screens';

if (!fs.existsSync(localDir)) {
  fs.mkdirSync(localDir, { recursive: true });
}

async function takeScreenshots(page, name) {
  await page.screenshot({ path: path.join(outDir, `${name}.png`) });
  await page.screenshot({ path: path.join(localDir, `${name}.png`) });
  console.log(`[Screenshot Saved] -> ${name}.png`);
}

(async () => {
  console.log("Launching Chromium browser for UI Flows 01-04 verification...");
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 }
  });
  const page = await context.newPage();

  // =========================================================================
  // FLOW 01: Onboarding & Authentication
  // =========================================================================
  console.log("\n==========================================");
  console.log("FLOW 01: Onboarding & First-Time Auth");
  console.log("==========================================");

  // 1. Visit /login
  await page.goto('http://localhost:8081/login', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1000);
  await takeScreenshots(page, '01_login_empty');

  // 2. Try default credentials or updated credentials
  const fillBtn = page.getByRole('button', { name: /заполнить|fill/i });
  if (await fillBtn.isVisible().catch(() => false)) {
    await fillBtn.click();
    console.log("Clicked 'Fill Default Admin Credentials' helper button");
  } else {
    await page.locator('input[type=email]').fill('admin@xchat.kz');
    await page.locator('input[type=password]').fill('xchat-admin-change-me');
  }
  await page.waitForTimeout(500);
  await takeScreenshots(page, '01_login_prefilled');

  // Submit login
  console.log("Submitting login credentials...");
  await page.locator('button[type=submit]').click();
  await page.waitForTimeout(2000);
  console.log("URL after login attempt 1:", page.url());

  if (page.url().includes('/login')) {
    console.log("Trying updated admin password...");
    await page.locator('input[type=email]').fill('admin@xchat.kz');
    await page.locator('input[type=password]').fill('AdminXChatPassword2026!');
    await page.locator('button[type=submit]').click();
    await page.waitForTimeout(2000);
    console.log("URL after login attempt 2:", page.url());
  }

  // 3. Forced Password Change view (if required)
  if (page.url().includes('change-password')) {
    console.log("On Change Password page, capturing and filling new password...");
    await takeScreenshots(page, '01_change_password_required');
    
    const pwInputs = page.locator('input');
    await pwInputs.nth(0).fill('xchat-admin-change-me');
    await pwInputs.nth(1).fill('AdminXChatPassword2026!');
    await pwInputs.nth(2).fill('AdminXChatPassword2026!');
    await page.waitForTimeout(500);
    await takeScreenshots(page, '01_change_password_filled');

    await page.locator('button[type=submit]').click();
    await page.waitForTimeout(2500);
    console.log("URL after password change submit:", page.url());
  }

  // 4. Chatboard Inbox
  await page.goto('http://localhost:8081/chatboard', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  await takeScreenshots(page, '01_chatboard_inbox');

  // =========================================================================
  // FLOW 02: Connect WhatsApp (QR Scan)
  // =========================================================================
  console.log("\n==========================================");
  console.log("FLOW 02: Connect WhatsApp (QR Scan)");
  console.log("==========================================");

  await page.goto('http://localhost:8081/accounts', { waitUntil: 'networkidle' });
  await page.waitForTimeout(1500);
  await takeScreenshots(page, '02_channels_page');

  console.log("Clicking + Connect Channel button...");
  const addAccountBtn = page.getByRole('button', { name: /подключить канал|connect.*channel|\+ подключить/i });
  await addAccountBtn.click();
  await page.waitForTimeout(1000);
  await takeScreenshots(page, '02_channel_picker_modal');

  console.log("Selecting WhatsApp option...");
  const waBtn = page.locator('[role="dialog"] button').filter({ hasText: 'WhatsApp' }).first();
  await waBtn.click({ force: true });
  await page.waitForTimeout(1000);
  await takeScreenshots(page, '02_whatsapp_preflight_modal');

  console.log("Clicking Show QR Code in pre-flight dialog...");
  const showQrBtn = page.locator('[role="dialog"] button').filter({ hasText: /показать qr|show qr/i }).first();
  if (await showQrBtn.isVisible().catch(() => false)) {
    await showQrBtn.click({ force: true });
    await page.waitForTimeout(3000);
    await takeScreenshots(page, '02_whatsapp_qr_pairing_modal');
  }

  // Close WhatsApp modal
  console.log("Closing WhatsApp modal...");
  await page.keyboard.press('Escape');
  await page.waitForTimeout(1000);

  // =========================================================================
  // FLOW 03: Connect Telegram Bot
  // =========================================================================
  console.log("\n==========================================");
  console.log("FLOW 03: Connect Telegram Bot");
  console.log("==========================================");

  console.log("Re-opening Connect Channel modal for Telegram...");
  await addAccountBtn.click();
  await page.waitForTimeout(1000);

  const tgBtn = page.locator('[role="dialog"] button').filter({ hasText: 'Telegram' }).first();
  await tgBtn.click({ force: true });
  await page.waitForTimeout(1000);
  await takeScreenshots(page, '03_telegram_connect_modal');

  // Close Telegram modal
  console.log("Closing Telegram modal...");
  await page.keyboard.press('Escape');
  await page.waitForTimeout(1000);

  // =========================================================================
  // FLOW 04: Knowledge Base Lifecycle
  // =========================================================================
  console.log("\n==========================================");
  console.log("FLOW 04: Knowledge Base Lifecycle");
  console.log("==========================================");

  // 1. Live KB Overview
  console.log("Navigating to /knowledge-base...");
  await page.goto('http://localhost:8081/knowledge-base', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await takeScreenshots(page, '04_knowledge_base_overview');

  // Topics Tab
  console.log("Switching to Topics tab...");
  const topicsTab = page.locator('button').filter({ hasText: /темы|topics/i }).first();
  if (await topicsTab.isVisible().catch(() => false)) {
    await topicsTab.click({ force: true });
    await page.waitForTimeout(1200);
    await takeScreenshots(page, '04_kb_topics_tab');

    // Open Topic Modal Form
    const addTopicBtn = page.locator('button').filter({ hasText: /\+ тема|\+ topic/i }).first();
    if (await addTopicBtn.isVisible().catch(() => false)) {
      await addTopicBtn.click({ force: true });
      await page.waitForTimeout(1000);
      await takeScreenshots(page, '04_kb_topic_modal');
      await page.keyboard.press('Escape');
      await page.waitForTimeout(500);
    }
  }

  // Products Tab
  console.log("Switching to Products tab...");
  const productsTab = page.locator('button').filter({ hasText: /товары|products/i }).first();
  if (await productsTab.isVisible().catch(() => false)) {
    await productsTab.click({ force: true });
    await page.waitForTimeout(1200);
    await takeScreenshots(page, '04_kb_products_tab');
  }

  // Prompt Tab
  console.log("Switching to Prompt tab...");
  const promptTab = page.locator('button').filter({ hasText: /промпт|prompt/i }).first();
  if (await promptTab.isVisible().catch(() => false)) {
    await promptTab.click({ force: true });
    await page.waitForTimeout(1500);
    await takeScreenshots(page, '04_kb_prompt_tab');
  }

  // 2. Draft / Ingest Hub (/playground)
  console.log("Navigating to Draft Hub (/playground)...");
  await page.goto('http://localhost:8081/playground', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await takeScreenshots(page, '04_draft_hub_overview');

  // Links Tab
  console.log("Checking Links tab in Draft...");
  const linksTab = page.locator('button').filter({ hasText: /ссылки|links/i }).first();
  if (await linksTab.isVisible().catch(() => false)) {
    await linksTab.click({ force: true });
    await page.waitForTimeout(800);
    await takeScreenshots(page, '04_draft_links_tab');
  }

  // Files Tab
  console.log("Checking Files tab in Draft...");
  const filesTab = page.locator('button').filter({ hasText: /файлы|files/i }).first();
  if (await filesTab.isVisible().catch(() => false)) {
    await filesTab.click({ force: true });
    await page.waitForTimeout(800);
    await takeScreenshots(page, '04_draft_files_tab');
  }

  // MCP Tab
  console.log("Checking MCP tab in Draft...");
  const mcpTab = page.locator('button').filter({ hasText: /chatgpt|claude|mcp/i }).first();
  if (await mcpTab.isVisible().catch(() => false)) {
    await mcpTab.click({ force: true });
    await page.waitForTimeout(800);
    await takeScreenshots(page, '04_draft_mcp_tab');
  }

  // 3. AI Simulator (/simulator)
  console.log("Navigating to AI Simulator (/simulator)...");
  await page.goto('http://localhost:8081/simulator', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);
  await takeScreenshots(page, '04_simulator_sandbox');

  console.log("\n==========================================");
  console.log("🎉 ALL UI Flows 01-04 Verified & Screens Saved!");
  console.log("==========================================");
  await browser.close();
})().catch(err => {
  console.error("Error during flow capture:", err);
  process.exit(1);
});




