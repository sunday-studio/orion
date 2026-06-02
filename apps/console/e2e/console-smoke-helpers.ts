import { type Page, expect } from "@playwright/test";

export const username = "admin";
export const password = "change-me";
export const coreApiURL = "http://127.0.0.1:18999/v1";
export const webhookReceiverURL = "http://127.0.0.1:19080";
export const coreURL = "http://127.0.0.1:18999";

export const signIn = async (page: Page) => {
  await page.goto("/login");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Enter" }).click();
  await expect(page.getByRole("heading", { name: "Incidents" })).toBeVisible();
};

export const authHeaders = async (page: Page) => {
  const token = await page.evaluate(() => localStorage.getItem("orion_token"));
  expect(token).toBeTruthy();
  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  };
};

export const createWebhookDestination = async (page: Page, name: string, url: string) => {
  await page.getByRole("button", { name: "New webhook" }).click();
  const dialog = page.getByRole("dialog", { name: "New webhook" });
  await dialog.getByPlaceholder("ops-webhook").fill(name);
  await dialog.getByPlaceholder("https://example.com/webhook").fill(url);
  await dialog.getByRole("button", { name: "Create destination" }).click();
  await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();
};

export const sendWebhookTest = async (page: Page, name: string) => {
  await page.getByLabel(`Open actions for ${name}`).click();
  await page.getByRole("menuitem", { name: "Send test" }).click();
};

export const openSettings = async (page: Page) => {
  await signIn(page);
  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
};
