The page presents the Knowledge Base as a collection of large editor cards. Each card follows the same visual language: white background, rounded corners (~16px), subtle border and shadow, plenty of whitespace, a colored icon in the top-left, a title with numbering, and a contextual three-dot menu in the top-right. Labels are light gray, inputs are rounded with a subtle border, and media is displayed as thumbnails with dashed "Add" placeholders. Every card is self-contained and vertically organized.

---

## 1. Product

The Product card is the most comprehensive entity because it combines structured information with multiple media types.

### General Information

* **Name** (text input)
* **Ref / Slug** (text input)
* **Price** (currency input)
* **Category** (text input)
* **Description** (multiline textarea)
* **In Stock** (toggle switch)

### Media

Media is divided into sections rather than one generic uploader.

#### Images

Horizontal thumbnail gallery.

* First image receives a **"Featured Image"** badge.
* Additional images appear as thumbnails.
* Dashed **Add** card at the end.

#### Videos

* Video thumbnail with duration overlay.
* Dashed **Add Video** placeholder.

#### Audio

Single upload area with **Add audio file** button.

#### Documents

Displayed as file chips/cards grouped by category.

Example groups:

* Certificates
* Instructions
* Warranty
* Specifications

Each document card shows:

* File icon
* Filename
* Size
* Remove action

---

## 2. Tariff

The Tariff card follows the same structure as Product but contains business-specific fields.

### Fields

* Name
* Ref
* Price
* Payment Type (select)
* Restrictions
* Commission / Fees
* Short Description

### Advantages

Tag editor.

Example:

* Fast
* Door Delivery

Each tag appears as a colored pill.

### Disadvantages

Separate tag editor.

Example:

* City Only

### Media

Exactly the same media sections as Product:

* Images
* Videos
* Documents

---

## 3. Delivery Zone

Unlike Product, this card is mostly form-based.

### Fields

* Name
* Ref
* Zone Level (dropdown)
* Parent Zone (dropdown)
* Delivery Available (toggle)
* Delivery Cost
* Delivery Time
* Notes (textarea)

The screenshot also demonstrates that multiple delivery zones can be shown in the same workspace as stacked cards.

The second example ("Байконур") uses the same layout.

No media is attached to delivery zones.

---

## 4. Contacts

A simple information editor.

### Fields

* Phone
* WhatsApp
* Email
* Website
* Instagram
* Address
* Working Hours
* Response Time
* Legal Information

### Media

Three dedicated blocks instead of a generic uploader:

#### Business Card

Preview card with company branding.

#### Map

Embedded location preview.

#### Documents

Small document cards.

Each media section has its own **Add** placeholder.

---

## 5. Topic (FAQ / Guide)

Represents articles or reusable knowledge.

### Fields

* Title
* Slug
* Keywords (tag editor)
* Content (rich text editor)

The editor toolbar contains basic formatting:

* Heading
* Bold
* Italic
* Lists
* Links

### Media

Separated into:

* Images
* Videos
* Documents
* Audio

Images support a featured image badge just like Products.

---

## 6. Policies

This card stores global business policies.

### Fields

* Shipping Cost
* Free Shipping Threshold
* Minimum Order
* Prepayment
* Installment
* Return Period
* Warranty
* Outside Delivery Zones

Most fields are single-line text inputs.

### Media

Only a **Documents** section is shown.

Example:

* commerce_policy_2026.pdf

---

## 7. Global Prompt

This card is visually different because it configures the assistant rather than business data.

### Fields

* Prompt Version
* Assistant Role
* Response Language (dropdown)
* Temperature (slider)
* Maximum Tokens
* Additional Instructions (textarea)

### Actions

Large footer buttons:

* Preview Prompt
* Save

The version badge ("Active") is displayed next to the prompt version.

---

# Shared Design Language

All cards consistently use the following UI patterns:

* **Large numbered title** with a colored icon indicating the entity type.
* **Three-dot overflow menu** in the top-right for contextual actions.
* **Rounded input fields** with subtle borders and consistent spacing.
* **Section dividers** separating metadata from media.
* **Horizontal media galleries** with image/video thumbnails and dashed "Add" placeholders.
* **Document attachments** rendered as compact file cards with filename and size.
* **Tag editors** represented as removable pill chips.
* **Toggles, dropdowns, sliders, and textareas** styled consistently across all entities.
* A light, airy layout with generous padding, making each card feel like an independent editor rather than a dense form.
