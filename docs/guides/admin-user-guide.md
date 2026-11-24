# HPay Admin: User Guide for Staff

Version: demo data, written 2026-11-24. English only.

---

## 1. What is HPay Admin, and who is this guide for?

HPay is a digital wallet. Customers send money, top up and pay bills in the HPay phone app.

**HPay Admin** is the website your team uses behind the scenes. You use it to check customer identity documents, review suspicious payments, approve special requests and watch that money moves correctly.

This guide is for back-office staff who have never used HPay Admin. You do not need any technical knowledge.

> **Tip**
> Read section 2 first, then section 3. After that, use the chapters in section 5 as a reference when you need a page.

---

## 2. Before you start

### 2.1 The safe rule

> **The safe rule**
> If you are not sure, press **Cancel**. Nothing is final until you press the red or green confirm button at the bottom of a window. Reading and clicking around the lists is always safe.

### 2.2 Open HPay Admin

1. Open your web browser (Chrome, Edge or Firefox).
2. Type the address your manager gave you. Press Enter.

*You should see: the HPay Admin login page (Figure 1).*

For the practice (demo) system the address is `http://localhost:5173`. Your manager will give you the real address.

### 2.3 Log in

1. Click the **Email** box and type your work email.
2. Click the **Password** box and type your password.
3. Click **Log in**.

*You should see: the Dashboard page, with a dark menu on the left (Figure 2).*

![Login page. Three red badges: 1 the Email box, 2 the Password box, 3 the Log in button.](admin/img/01-login.png)

*Figure 1: The login page. Badge 1 is Email, badge 2 is Password, badge 3 is Log in.*

> **Careful**
> After 5 wrong passwords your account is locked for 15 minutes. Wait, then try again. Do not keep guessing.

### 2.4 Log out

1. Look at the top right corner of the page.
2. Click the small arrow-out icon next to your name (badge 3 in Figure 2). Its hover text is **Log out**.

*You should see: the login page again.*

Log out when you leave your desk. Your session also ends when you close the browser tab.

### 2.5 If the page shows an error or you are logged out

- **You are sent back to the login page.** Your session ran out. Log in again. You return to the page you were on.
- **A red message says "Could not load".** Press the **Refresh** button on that page, or press F5 on your keyboard. If it stays, tell your contact (section 8).
- **"No access".** Your role cannot open that page. See section 4.
- **"Page not found" (404).** The address is wrong. Click **Back to Dashboard**.

### 2.6 How the screen is laid out

![Dashboard with the left menu, top bar and Log out icon marked.](admin/img/02-dashboard.png)

*Figure 2: The Dashboard. Badge 1 is the left menu. Badge 2 is the Needs attention entry. Badge 3 is Log out. Badge 4 is the first alert card.*

- **Left menu (badge 1):** one entry for each page you can use. It only shows pages your role may open.
- **Top bar:** the page name, your name, your role and **Log out**.
- **Main area:** the page itself.
- A **red number** next to **Needs attention** in the menu means cases are waiting for a person.

---

## 3. Your first 10 minutes

Do these steps in order. They only read information, so they are safe. Use the practice system with the demo data.

1. Log in (section 2.3). *You should see: the Dashboard.*
2. Look at the three cards under **Needs attention**. *You should see: counts such as "1" under **Pending KYC** and **Open Fraud flags**.* Click **Pending KYC**.
3. On **KYC Review**, click **Open →** on the first row. *You should see: an ID photo, a selfie and notes from the AI.* Click **← Back**. Do not approve anything yet.
4. Click **Fraud Monitor** in the left menu. *You should see: one open flag in a table, with a red **Open** status.* Click the flag number to read its explanation below the table.
5. Click **Workflow** in the left menu. *You should see: a list of payments with a **Status** for each, such as **Failed**, **Completed**.* Click **Open →** on any row to see its steps.
6. Click **My Tasks**. *You should see: requests waiting for your decision, with a **Process** button on each row.* Do not press **Approve** or **Reject** while practising.
7. Click **Needs attention**. *You should see: "Nothing needs attention." That is the good result.*
8. Click **Audit Log**. *You should see: a table with columns Time, Actor, Action. This is the record of what staff did.*
9. Click **Log out** (top right).

Well done. You now know every kind of screen in HPay Admin.

> **Not available yet**
> In the demo system the **Audit Log** table can be empty ("Nothing here yet"). The page still works.

---

## 4. Roles and permissions

Your **role** decides which pages you can open and which buttons work. Your role name is shown at the top right, next to your name.

| Role | Can do | Menu items you will see |
|---|---|---|
| **Superadmin** | Everything: review KYC, resolve fraud flags, handle workflows and approvals, run reconciliation, edit organization policies, manage AI settings, change services. | All 12 |
| **Ops** | Handle workflows and approvals, run reconciliation. | Dashboard, KYC Review, Workflow, My Tasks, Needs attention, Reconciliation, Organizations & Policy, Audit Log |
| **KYC Reviewer** | Approve or reject identity (KYC) requests. | Dashboard, KYC Review, Workflow, My Tasks, Audit Log |
| **Risk Officer** | Resolve fraud flags, handle workflows and approvals. | Dashboard, KYC Review, Fraud Monitor, Workflow, My Tasks, Needs attention, Organizations & Policy, Audit Log |
| **Viewer** | Look only. No buttons that change anything. | Dashboard, KYC Review, Workflow, My Tasks |

How this table was built: from the role list in the admin app code, as it works today. The roles can change; ask a Superadmin if your menu looks different.

**What "View only" means.** You can see the information but cannot change it. A button may be greyed out, or a row may say **View only** instead of a **Process** button. Hover over a greyed button to read the reason. It says: "Your role does not have permission for this action."

![Menu of an Ops user, which has fewer entries than a Superadmin.](admin/img/03-ops-menu.png)

*Figure 3: What an Ops user sees. Badge 1 is the shorter menu: no Fraud Monitor, Service Catalog, Offer Factory or AI Settings. Badge 2 is the role name.*

![A "No access" panel.](admin/img/22-no-access.png)

*Figure 4: If you type the address of a page your role cannot open, you see "No access".*

> **Careful**
> Some pages in the menu open for roles that cannot change anything there. For example, an Ops user can open **KYC Review** but sees "your role does not allow this action" instead of the list. This is how the product works today, not a mistake on your side.

---

## 5. The pages, one by one

### 5.1 Dashboard

**What this page is for.** A one-screen summary of what needs your attention.

**When you need it.** Open it first thing in the morning.

**Task: Find out what is waiting**

1. Click **Dashboard** in the left menu. *You should see: four number cards and three alert cards.*
2. Read the alert cards under **Needs attention**: **Pending KYC**, **Open Fraud flags**, **Workflows stuck > 5 min**.
3. Click a card. *You should see: the matching page opens.*

| Card | What it means |
|---|---|
| **Pending KYC** | Identity requests that need a person to decide. |
| **Open Fraud flags** | Suspicious payments nobody has handled yet. |
| **Workflows stuck > 5 min** | Payments that have not moved for more than 5 minutes. |
| **New users (today)**, **Transactions (24h)**, **Volume (24h)**, **Failure rate** | Daily numbers. |

> **Not available yet**
> The four daily-number cards and the **Volume by hour** chart show "—" and "(no analytics API yet)". They are not connected to data. Also, **Workflows stuck > 5 min** shows "—" with "API coming in A3". Use the **Workflow** page and its **Stuck > 5 min** button instead.

**If something looks wrong.** If a card says "Could not load this count", your role cannot see that information, or the system is busy. Press F5. If a Viewer or Ops user sees this on the first two cards, that is normal for their role.

---

### 5.2 KYC Review

**What this page is for.** Check each customer's identity documents (KYC) and approve or reject them. KYC means "Know Your Customer".

**When you need it.** When **Pending KYC** on the Dashboard is above zero.

![KYC list with the Pending tab, the search box and the Open link marked.](admin/img/04-kyc-list.png)

*Figure 5: The KYC list. Badge 1 is the Pending tab. Badge 2 is the search box. Badge 3 is Open →.*

**Task A: Approve a request**

1. Click **KYC Review** in the left menu. *You should see: the list, with the **Pending** tab selected.*
2. Click **Open →** at the end of a row (badge 3 in Figure 5). *You should see: the ID card photo, the selfie, the registration details and the AI analysis (Figure 6).*
3. Compare the name and face with the photos. Read the **AI analysis** notes.
4. Click **✓ Approve (A)**. *You should see: a green message "Application approved." and the next request opens.*

![KYC detail page showing the ID photos, details, AI analysis and the Approve and Reject buttons.](admin/img/05-kyc-detail.png)

*Figure 6: One request. Badge 1 is the AI analysis. Badge 2 is the registration details. Badge 3 is Approve. Badge 4 is Reject. Badge 5 is Back.*

> **Careful**
> **✓ Approve (A)** works at once. There is no "Are you sure?" window. Check the photos before you click. Do not press the **A** key by accident.

**Task B: Reject a request**

1. Open the request as in Task A.
2. Click **✕ Reject (R)**. *You should see: a window titled "Reject KYC application" (Figure 7).*
3. Click the big box and type the reason. It needs at least 10 characters. The reason is saved to the Audit log.
4. Click **Confirm rejection** (badge 3). *You should see: a green message "Application rejected."*

To stop, click **Cancel** (badge 2) instead. Nothing changes.

![The reject window with the reason box, Cancel and Confirm rejection.](admin/img/06-kyc-reject-dialog.png)

*Figure 7: The reject window. Badge 1 is the reason box. Badge 2 is Cancel. Badge 3 is Confirm rejection.*

**Task C: Find a customer**

1. Click the search box (badge 2 in Figure 5).
2. Type part of the name or phone number. *You should see: the list shrinks.*

**What the columns and tabs mean**

| Item | Meaning |
|---|---|
| **Pending** tab | Needs a person to decide. |
| **Auto-approved by AI** tab | The AI approved it. Look only. |
| **Rejected** tab | Already rejected. Look only. |
| **AI Score** | 0 to 100. Higher means the documents look more genuine. Green is 70 or more, amber 40 to 69, red below 40. |
| **AI Verdict** | The AI's suggestion, shown as a word badge, such as manual_review (a person must decide). |
| **Waiting** | How long the request has been in the queue. |

Keyboard shortcuts exist (J and K move up and down, Enter opens, A approves, R rejects). You can ignore them.

> **Careful**
> Only **Pending** requests can be decided. If another person already handled one, you see "This application was already handled elsewhere." Nothing is lost.

**If something looks wrong.** If the photos are blank, press F5; the photo links expire after a short time. If the list is empty, the queue is clear.

---

### 5.3 Fraud Monitor

**What this page is for.** Review payments the system marked as suspicious (a "fraud flag") and decide what to do.

**When you need it.** When **Open Fraud flags** on the Dashboard is above zero.

![Fraud Monitor with the status filter, the flag number and the two action buttons marked.](admin/img/07-fraud-list.png)

*Figure 8: The fraud list. Badge 1 is the status filter. Badge 2 is the flag number. Badge 3 is Mark as safe. Badge 4 is Confirm fraud.*

**Task A: Read a flag**

1. Click **Fraud Monitor** in the left menu. *You should see: one row per flag.*
2. Click the flag number, for example **#c751** (badge 2). *You should see: its explanation below the table. It lists the rules that triggered and an "AI assessment".*

**Task B: Mark a flag as safe**

1. Read the flag (Task A).
2. Click **✓ Mark as safe** (badge 3). *You should see: a small browser window asking "Mark flag ... as safe?"*
3. Click **OK** to confirm, or **Cancel** to stop. *You should see: the flag turns **Cleared**.*

**Task C: Confirm fraud and choose a consequence**

1. Click **🔒 Confirm fraud — choose consequence…** (badge 4). *You should see: a window "Confirm resolution" with three choices (Figure 9).*
2. Click one choice (badges 1 and 2 show two of them).
3. Type a reason in the box (badge 3). It needs at least 10 characters.
4. Click **Confirm Fraud resolution** (or **Mark as safe** if you chose the third option). To stop, click **Cancel** (badge 4).

![The fraud resolution window with three choices, a reason box and Cancel.](admin/img/08-fraud-resolve-dialog.png)

*Figure 9: The three consequences. Badge 1 is "Lock Account + cancel Pending Transactions". Badge 2 is "Mark as safe / Record only, no action". Badge 3 is the reason box. Badge 4 is Cancel.*

| Choice | What happens |
|---|---|
| **Lock Account + cancel Pending Transactions** | The customer cannot log in. Payments still running are undone. Finished payments stay. |
| **Lock Account + reverse Completed Transaction** | Also creates a reversal request for the finished payment. Needs the Risk Officer role. |
| **Mark as safe / Record only, no action** | Keeps a record. No effect on the customer. |

> **Careful**
> The lock choices affect a real customer's access to their money. The window says it "cannot be undone from the UI". If unsure, press **Cancel** and ask a Risk Officer.

**What the columns mean**

| Column | Meaning |
|---|---|
| **Flag** | The flag's short number. |
| **Rule** | Why it was flagged: **VELOCITY_5MIN** (many payments in 5 minutes), **AMOUNT_SPIKE** (much bigger than usual), **NEW_DEVICE** (new phone), **ODD_HOURS** (middle of the night). |
| **Score** | Risk from 0 to 100. 80 or more is red. |
| **Age** | Time since the flag was raised. "⚠ SLA" means a high-risk flag has waited too long. Handle it first. |
| **Status** | **Open** (not handled), **Cleared** (marked safe), **Blocked** (fraud confirmed). |

Other controls: **Status: Open** filter, **Rule: All** filter and **⟳ Refresh**. You can tick boxes in the table and click **☑ Mark safe (n)** to clear several flags at once. Use it only when you have read every selected flag.

> **Not available yet**
> The **User** column can show "—". The page title shows a technical phrase, "(TanStack Table v8)". Ignore it.

**If something looks wrong.** If the buttons are grey, your role cannot resolve flags (section 4). If you see an error after confirming, the flag was probably handled by someone else. Press **⟳ Refresh**.

---

### 5.4 Workflow

**What this page is for.** Watch each payment as it moves through its steps. A "workflow" is the list of steps a payment goes through, such as check the user, check for fraud, move the money, confirm. One run of those steps is a "workflow instance".

**When you need it.** When a payment looks stuck or failed.

![The workflow list with the Stuck filter, status filter and Open links marked.](admin/img/09-workflow-list.png)

*Figure 10: The workflow list. Badge 1 is the "Stuck > 5 min" button. Badge 2 is the status filter. Badge 3 is Open →.*

**Task A: Find stuck payments**

1. Click **Workflow** in the left menu.
2. Click **Stuck > 5 min** (badge 1). *You should see: only payments that have not moved for 5 minutes.*

**Task B: See why a payment failed**

1. Click **Open →** on the row (badge 3). *You should see: the payment details and a step timeline (Figure 11).*
2. Read the **TIMELINE STEP EXECUTIONS**. The step that failed is marked.

**Task C: Retry a failed step**

1. Open the payment (Task B).
2. Click the box under **Reason for this action (required):** and type why (badge 2 in Figure 11).
3. Click **⟳ Retry** followed by the step name (badge 3). *You should see: a green message "Retry command sent. The system will retry the current step."*

**Task D: Undo a payment (compensate)**

1. Open the payment and type the reason as in Task C.
2. Click **↩ Compensate now (reverse entries)** (badge 4). *You should see: "Compensate command sent. Earlier ledger entries have been reversed."*

The button **Force fail + compensate** at the top right does the same as **Compensate now**. "Compensate" means the system undoes the steps already done, for example by giving the money back to the sender.

![A workflow detail page with the timeline, the reason box and the Retry and Compensate buttons marked.](admin/img/10-workflow-detail.png)

*Figure 11: One payment. Badge 1 is the timeline. Badge 2 is the reason box. Badge 3 is Retry. Badge 4 is Compensate now. Badge 5 is Monitor (back).*

> **Careful**
> **Compensate** moves money back. It happens at once, with no confirm window. Use it only when told to, and always write a clear reason. **Retry** only repeats the failed step.

**What the statuses mean**

| Status | Meaning |
|---|---|
| **Running** | In progress. |
| **Waiting** | Waiting for something outside, such as an approval. |
| **Completed** | Finished correctly. |
| **Failed** | A step failed. Needs a person. |
| **Compensated** | Failed, and the money was given back. |
| **Age** | Time since it started. Red means stuck. |

The list refreshes by itself every 10 seconds. The raw-data box on the detail page is for technical staff; ignore it.

**If something looks wrong.** If **Retry** and **Compensate now** are grey, your role cannot act (Viewer). If the failure repeats, do not retry again and again. Tell your contact (section 8) with the Transaction code.

---

### 5.5 My Tasks

**What this page is for.** Your inbox of special requests that need a decision, such as reversing a payment or locking an account. This is the "maker-checker" idea: one person asks (the maker), a different person approves (the checker).

**When you need it.** When your manager asks you to "clear the approvals".

![My Tasks inbox with the request row, approval level and Process button marked.](admin/img/11-tasks-list.png)

*Figure 12: The inbox. Badge 1 is the order type. Badge 2 is the approval level. Badge 3 is Process.*

**Task: Approve or reject a request**

1. Click **My Tasks** in the left menu. *You should see: one row per waiting request.*
2. Read the **Order type**, **Amount** and **Request reason**.
3. Click **Process** at the end of the row. *You should see: a window "Process order" (Figure 13).*
4. Type your reason in **Decision reason**. It needs at least 10 characters.
5. Click **Approve** or **Reject**. *You should see: a green message "Order approved." or "Order rejected."*

To stop, click **Cancel**.

![The Process order window with the reason box and Cancel, Reject and Approve buttons.](admin/img/12-tasks-dialog.png)

*Figure 13: The decision window. Badge 1 is the reason box. Badge 2 is Cancel. Badge 3 is Reject. Badge 4 is Approve.*

> **Careful**
> **Approve** on a **reversal** gives money back to a customer. **Approve** and **Reject** work at once. If you press the wrong one, you cannot take it back from this page. Read the amount and reason first.

| Column | Meaning |
|---|---|
| **Order type** | The kind of request: **reversal** (give money back), **account_lock** (lock a customer), or a policy change. |
| **Transaction code** | The payment the request is about. |
| **Approval level** | **L1**, **L2** and so on. Bigger requests need more levels of approval. |
| **Expires** | After this time the request is closed without a decision. |

**If something looks wrong.** If a row says **View only** instead of **Process**, your role cannot decide (Figure 14). If you see "handled elsewhere", someone else decided first; the list updates.

![My Tasks as seen by an Ops user.](admin/img/24-tasks-ops.png)

*Figure 14: Ops users can press Process. A Viewer sees "View only" there instead.*

---

### 5.6 Needs attention

**What this page is for.** A list of money cases that the system parked for a person. It lists two kinds: approvals the system could not apply (**needs_manual**) and payments whose undo step failed (**compensation_failed**).

**When you need it.** When the red number appears next to **Needs attention** in the menu.

**Task: Check for parked cases**

1. Click **Needs attention** in the left menu. *You should see: "Open cases: 0" and "Nothing needs attention." when all is well (Figure 15).*
2. If rows appear, read the **Ref code**, **Amount**, **Age** and **Reason** columns.
3. Click **Detail** (for a payment) or **Audit log** (for an approval) to read more.

![Needs attention page showing the empty state.](admin/img/13-attention.png)

*Figure 15: The page when nothing is parked. Badge 1 is the menu entry. Badge 2 is the "Nothing needs attention." message.*

> **Careful**
> This page only shows cases. It cannot fix them and it moves no money. A parked case is serious. Tell your contact (section 8) with the Ref code.

| Column | Meaning |
|---|---|
| **Type** / **Workflow** | Which kind of case. |
| **Ref code** | The payment's code. Quote it when you ask for help. |
| **Age** | Time since it was parked. |
| **Reason** | Why it needs a person. |

> **Not available yet**
> There is no button here to resolve a case. How a parked case is solved is still being decided.

**If something looks wrong.** The number in the menu updates about every 30 seconds.

---

### 5.7 Reconciliation

**What this page is for.** Compare HPay's records of top-ups with the bank's statement, day by day. "Reconciliation" means checking that two lists agree.

**When you need it.** Once a day, or when finance asks you to re-check a date. The system also runs by itself each night.

![Reconciliation with the date box, Run Reconciliation and Export Excel marked.](admin/img/14-recon.png)

*Figure 16: Reconciliation. Badge 1 is the date box. Badge 2 is Run Reconciliation. Badge 3 is Export Excel.*

**Task A: Look at one day**

1. Click **Reconciliation** in the left menu.
2. Click the date box (badge 1) and choose a date. *You should see: the results for that day, or "Nothing here yet" if it has not run.*

**Task B: Run a check for a day**

1. Choose the date.
2. Click **⟳ Run Reconciliation** (badge 2). *You should see: a message "Reconciliation completed for" and the date, then the table fills.*

**Task C: Get the Excel file**

1. Choose a date that has results.
2. Click **⤓ Export Excel** (badge 3). *You should see: the file opens in a new tab. If not, a message says "No Reconciliation file for ... yet."*

> **Careful**
> A day can be run only once. Running the same date again gives an error. That is normal and safe.

| Result | Meaning |
|---|---|
| **matched** | HPay and the bank agree. |
| **amount_mismatch** | Both have the payment but the amounts differ. |
| **missing_external** | HPay has the payment but the bank statement does not. |
| **missing_internal** | The bank has it but HPay does not. |

The top cards show **Total reconciled**, **Matched**, **Mismatched** and **Run time**. An amber badge shows how many items are mismatched. A purple "AI root-cause summary" may appear when there are mismatches.

> **Not available yet**
> The last column, **Resolve**, only shows a pop-up message. It does not resolve anything yet. The date box starts at a fixed demo date, 2026-07-07, not today's date.

**If something looks wrong.** The **⟳ Run Reconciliation** button is grey if your role cannot run it. Nothing here is a problem if the table says "Nothing here yet" for a day with no top-ups.

---

### 5.8 Service Catalog

**What this page is for.** See every service HPay offers (transfer, bill payment and so on) and switch one into maintenance.

**When you need it.** When a service has a problem and customers must not use it for a while.

![Service Catalog with the search box, maintenance button, pipeline button and Add service marked.](admin/img/15-services.png)

*Figure 17: The service list. Badge 1 is the search box. Badge 2 is Put in maintenance. Badge 3 is the pipeline button. Badge 4 is Add service.*

**Task A: Look at a service**

1. Click **Service Catalog** in the left menu.
2. Click **⚡ Low-Code Pipeline** (badge 3) on a row. *You should see: a window "Transaction Design Manager" with tabs (Figure 18).*
3. Click **Close** when done.

**Task B: Put a service in maintenance, then back**

1. Click **Put in maintenance ⚡** on the row (badge 2). *You should see: a browser window "Confirm changing the status of service ... to: MAINTENANCE?"*
2. Click **OK** to confirm or **Cancel** to stop. *You should see: the Status becomes **Maintenance** (amber).*
3. To restore, click **Reactivate →** on the same row, then **OK**. *You should see: the Status is **Active** again.*

![The Transaction Design Manager window for one service, opened from the pipeline button.](admin/img/16-service-pipeline.png)

*Figure 18: The service window. Use the tabs to look only. Click Close to leave.*

> **Careful**
> While a service is in **Maintenance**, customers cannot use it. Do it only on instruction. Always put it back with **Reactivate →** afterwards.

| Status | Meaning |
|---|---|
| **Active** | Customers can use it. |
| **Maintenance** | Paused on purpose. |
| **Disabled** | Switched off. |

> **Not available yet**
> **+ Add service** is grey: HPay cannot create a service from this page yet. In the service window the **Save transaction design** button is also grey: the pipeline can be read but not changed. An amber banner at the top of the page says so.

**If something looks wrong.** Only a Superadmin can change a service's status. If **Put in maintenance ⚡** is grey, that is why.

---

### 5.9 Offer Factory

**What this page is for.** See the customer and business plans (offers), such as "Personal (KYC tier 1)", and the limits that come with them. A **plan** (offer) decides what a customer may do and how much, based on how well HPay knows them.

**When you need it.** When someone asks "what is the limit for this plan?"

![Offer Factory with the New plan button and Configure links marked.](admin/img/17-offers.png)

*Figure 19: The plan list. Badge 1 is New plan. Badge 2 is Configure →.*

**Task: Read a plan's limits**

1. Click **Offer Factory** in the left menu. *You should see: a table of plans.*
2. Click **Configure →** on a row (badge 2). *You should see: the plan page (Figure 20).*
3. Read **AUTHORIZED SERVICES**, **MAX PER TRANSACTION**, **DAILY LIMIT**, **MONTHLY LIMIT** and **Authentication matrix**.
4. Click **← Back to Plans**.

![A plan page with the services, limits and the grey Save Plan settings button.](admin/img/18-offer-detail.png)

*Figure 20: One plan. Badge 4 is Save Plan settings (grey). Badge 5 is Back to Plans.*

| Column | Meaning |
|---|---|
| **Plan code** | The plan's short name. |
| **Target role** | **customer** (personal) or **merchant** (business). |
| **Level** | L0, L1, L2... The higher, the more the customer may do. This is the "tier". |
| **Authorized services** | How many services the plan can use. |
| **Status** | **Active** or **Draft**. |

The **Authentication matrix** says what proof a customer must give: **NONE**, **PIN**, **OTP** (a code by text message) or **2FA** (PIN and code).

> **Not available yet**
> You cannot save changes to a plan yet. **Save Plan settings** and **+ New plan** do nothing. The numbers on the plan page are a preview only. In the demo, many plans show an empty "Authorized services" count and **Draft** status.

**If something looks wrong.** The page opens only for a Superadmin. Others see "No access".

---

### 5.10 Organizations & Policy

**What this page is for.** See each business customer ("organization") and the rules for approving its payments. A **policy** says how many approvers (checkers) must sign a payment of a given size.

**When you need it.** When a business asks why a large payment needs two approvals.

![Organizations page with the organization list, levels, Add level and Save marked.](admin/img/19-orgs.png)

*Figure 21: The policy page. Badge 1 is the organization. Badge 2 is a level. Badge 3 is Add level. Badge 4 is Save. Badge 5 is Discard changes.*

**Task A: Read a policy**

1. Click **Organizations & Policy** in the left menu.
2. Click the organization in the left list (badge 1). *You should see: its policy. For example "Level 1: From 10M, needs 1 Checker L1 signature".*

**Task B: Change how many signatures a level needs**

1. Click **−** or **+** next to the level (badge 2). *You should see: the number changes.*
2. Click **Save — create version v4** (badge 4). *You should see: a message "Saved new approval policy (version v4)".*

To undo before saving, click **Discard changes** (badge 5).

> **Careful**
> Saving creates a new version for **new** orders only. Orders already waiting keep the old rule. The yellow note on the page says how many that is. **Save** applies at once.

> **Not available yet**
> **+ Add policy** only shows a pop-up message. It does not create a policy. The card "Regular payments (v1)" and the heading "Large payments (v3)" are fixed text.

**If something looks wrong.** If you see "Your role does not have permission for this action. (orgs.edit)", you can read the page but not change it (Figure 22). Only a Superadmin can edit.

![Organizations page as seen by an Ops user, with the permission note and greyed buttons.](admin/img/23-orgs-view-only.png)

*Figure 22: View only. Badge 1 is the permission note. Badge 2 is the grey Add policy button. Badge 3 is the grey Save button.*

---

### 5.11 AI Settings

**What this page is for.** Choose which AI service reads ID documents and explains fraud flags, and store the secret key it needs.

**When you need it.** Rarely. Only a Superadmin should change it.

![AI Settings with the keys list, the Add a new API key form and the routing section marked.](admin/img/20-ai-settings.png)

*Figure 23: AI Settings. Badge 1 is the key list. Badge 2 is the Add a new API key form. Badge 3 is Model routing by function.*

**Task: Read the current setup**

1. Click **AI Settings** in the left menu.
2. Read **AI API keys**: each key shows **ACTIVE** or **INACTIVE**.
3. Read **Model routing by function**: for each job (KYC Scoring, Fraud Advising) it shows the AI provider in use.

Changing settings is for experts. The page has **+ Save new key** and **Save routing** buttons.

> **Careful**
> Turning off a key that is in use makes that AI job fail. The page warns you first ("Confirm disabling API key"). If you are not told to do this, do not touch this page. The secret key is like a password: never write it in an email.

**What the words mean**

| Word | Meaning |
|---|---|
| **Provider** | The company running the AI (Anthropic, OpenAI) or **Mock Engine** (a fake one for practice). |
| **Model Name** | Which version of the AI to use. |
| **API key** | A secret code that lets HPay use the AI service. |

**If something looks wrong.** Only a Superadmin can open this page. Others see "No access".

---

### 5.12 Audit Log

**What this page is for.** The record of what staff did and what happened, with who, when and why.

**When you need it.** When someone asks "who approved this?" or "when was this rejected?"

![Audit Log with the search box and column headings marked.](admin/img/21-audit.png)

*Figure 24: The audit log. Badge 1 is the search box. Badge 2 is Actor. Badge 3 is Event details.*

**Task: Find an entry**

1. Click **Audit Log** in the left menu.
2. Click the search box (badge 1) and type a word, such as a reason or a transaction code. *You should see: only matching rows.*

| Column | Meaning |
|---|---|
| **Time** | When it happened. |
| **Actor** | Who did it. |
| **Action** | What was done. |
| **Resource** | What it was done to. |
| **Event details** | The reason or extra information. |
| **IP Source** | The computer address it came from. |

The log cannot be edited. It is read-only for everyone.

> **Not available yet**
> In the demo system the table is empty ("Nothing here yet"), so no rows can be shown in Figure 24.

**If something looks wrong.** Viewers cannot open this page. Ask a Superadmin to look up an entry for you.

---

## 6. Glossary

| Word | Plain meaning |
|---|---|
| **KYC** | "Know Your Customer". Checking a customer's identity with an ID card photo and a selfie. |
| **Fraud flag** | A warning that a payment looks suspicious. A person decides if it is real. |
| **Maker** | The person who asks for something to be done. |
| **Checker** | The person who approves or rejects what the maker asked. A different person from the maker. |
| **Approval level (L1, L2)** | Large requests need several rounds of approval. L1 signs first, then L2, and so on. |
| **Reconciliation** | Checking that HPay's records agree with the bank's records for a day. |
| **Workflow instance** | One payment as it moves through its steps. |
| **Stuck** | A workflow instance that has not moved for more than 5 minutes. |
| **Compensation** | Undoing the steps a payment already did, for example giving the money back. |
| **Reversal** | A request to give money back for a finished payment. It needs approval. |
| **Plan / offer** | A package of services and limits given to a customer. |
| **Service** | One thing customers can do, such as send money or pay a bill. |
| **Policy** | The rule saying how many approvers a payment of a given size needs. |
| **Tier / level** | How much HPay knows a customer. Higher level means higher limits. |
| **Organization** | A business customer with several staff using HPay. |
| **Audit log** | The permanent list of who did what and why. |
| **API** | A technical link between two programs. It appears on screens in messages like "no analytics API yet". It means the data feed is not built yet. |
| **Role** | Your job type in HPay Admin. It decides what you can do. |

---

## 7. FAQ and troubleshooting

**I cannot log in.** Check that Caps Lock is off. Check your email spelling. If the message says the account is locked, wait 15 minutes. After 5 wrong tries the lock starts. Still stuck? See section 8.

**I was logged out suddenly.** Sessions end after a short time (the sign-in is valid for about 15 minutes, as set in the code today). Log in again. You return to the page you were on.

**A button is grey.** Your role cannot use it. Hover over it for the reason. Read section 4.

**I see "Not available yet", "API coming" or "no analytics API yet".** That part of HPay Admin is not built yet. It is not your mistake. Use the other pages named in this guide.

**The list is empty.** "Nothing here yet" or "No fraud flags" usually means all is clear. Check the tab and filter at the top (for example **Status: Open**).

**I approved the wrong item.** Stay calm. Do not try to fix it yourself. Write down the item, the time and what you did. Contact your manager at once (section 8). Your action is saved in the Audit log with your name.

**The page looks wrong or stuck.** Press F5 to reload. If it still looks wrong, log out and in again.

**I cannot find a menu item.** Your role does not include it (section 4).

---

## 8. Who to contact

> **TO BE FILLED IN BY THE OWNER BEFORE SENDING TO STAFF**
>
> - Support contact name: `________________`
> - Phone or chat: `________________`
> - Email: `________________`
> - Office hours: `________________`
> - Out of hours (urgent money problem): `________________`
>
> When you ask for help, tell them: your name, the page, what you clicked, the exact message on screen and the time.
