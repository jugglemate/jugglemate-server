## ADDED Requirements

### Requirement: Console web defaults to Simplified Chinese
The console web application SHALL use `zh-CN` as the default interface language when no stored language preference exists.

#### Scenario: First visit without stored preference
- **WHEN** a user opens the console for the first time and no language preference is stored locally
- **THEN** the interface displays in Simplified Chinese (`zh-CN`)

#### Scenario: Stored preference takes precedence
- **WHEN** a user has previously selected `en-US` and the preference is stored locally
- **THEN** the console opens in English (`en-US`) instead of the default

### Requirement: Console web supports Chinese and English switching
The console web application SHALL allow users to switch the interface language between Simplified Chinese (`zh-CN`) and English (`en-US`).

#### Scenario: Switch from Chinese to English
- **WHEN** the current language is `zh-CN` and the user selects English from the language control
- **THEN** user-visible UI text updates to English without a full page reload beyond normal React re-render

#### Scenario: Switch from English to Chinese
- **WHEN** the current language is `en-US` and the user selects Chinese from the language control
- **THEN** user-visible UI text updates to Simplified Chinese

### Requirement: Language preference is persisted locally
The console web application SHALL persist the selected interface language in browser local storage and SHALL restore it on subsequent visits.

#### Scenario: Preference survives refresh
- **WHEN** a user selects `en-US` and refreshes the browser on any console route
- **THEN** the interface remains in English

#### Scenario: Preference survives new session
- **WHEN** a user selects `zh-CN`, closes the tab, and opens the console again in a new tab
- **THEN** the interface remains in Simplified Chinese

### Requirement: Ant Design locale follows interface language
The console web application SHALL configure Ant Design `ConfigProvider` locale to match the active interface language.

#### Scenario: Chinese Ant Design widgets
- **WHEN** the active language is `zh-CN`
- **THEN** Ant Design built-in component text (for example pagination and empty states) uses Chinese locale

#### Scenario: English Ant Design widgets
- **WHEN** the active language is `en-US`
- **THEN** Ant Design built-in component text uses English locale

### Requirement: Document language attribute follows interface language
The console web application SHALL set `document.documentElement.lang` to match the active interface language.

#### Scenario: Chinese document lang
- **WHEN** the active language is `zh-CN`
- **THEN** `document.documentElement.lang` is `zh-CN`

#### Scenario: English document lang
- **WHEN** the active language is `en-US`
- **THEN** `document.documentElement.lang` is `en-US`

### Requirement: Language control is available in the main layout
The console web application SHALL expose a language switch control in the authenticated main layout header alongside existing global controls.

#### Scenario: Language control visible when logged in
- **WHEN** an authenticated user views any main layout page
- **THEN** a language switch control is visible in the header

#### Scenario: Current language is indicated
- **WHEN** the language switch control is displayed
- **THEN** the control indicates which of `zh-CN` or `en-US` is currently active

### Requirement: Core console surfaces are translated
The console web application SHALL render user-visible text for core surfaces through the i18n translation system in both supported languages.

#### Scenario: Navigation labels translated
- **WHEN** the user switches language on pages with sidebar navigation
- **THEN** menu labels for Dashboard, Users, and Inboxes display in the active language

#### Scenario: Auth pages translated
- **WHEN** the user opens login or register pages
- **THEN** headings, labels, buttons, and validation messages display in the active language

#### Scenario: Error pages translated
- **WHEN** the user navigates to 403 or 404 routes
- **THEN** page titles and primary actions display in the active language
