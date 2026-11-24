# Writing rules for HPay user guides

Rules applied to `docs/guides/admin-user-guide.md` and to any future guide for non-technical staff. They were distilled on 2026-11-24 from the public sources below. Those pages were read as reference only; nothing was downloaded, installed or run from them.

## Rules (apply all of them)

1. **Write for the real reader.** Assume a back-office worker who has never seen the product, may read English as a second language, and fears clicking the wrong thing. Use words they already know. (plainlanguage.gov)
2. **Keep sentences short.** Aim for under 20 words, one idea per paragraph, subject and verb close together. (plainlanguage.gov)
3. **Use active voice and "you".** Say "Click **Approve**", not "The item should be approved". (plainlanguage.gov, Google style guide)
4. **One action per numbered step.** Start each step with a verb in the imperative. If a step feels long, split it. (Microsoft Style Guide, Google style guide)
5. **Keep a procedure short.** Aim for at most seven steps. Split longer tasks into two procedures. (Microsoft Style Guide)
6. **Name each control exactly as it appears on screen**, in **bold**, including its capital letters and symbols.
7. **Say what happens next.** End each step that changes the screen with an italic *You should see:* line. (Microsoft Style Guide: tell the reader where to perform the action and what results)
8. **Title each procedure by the goal**, in parallel wording ("Approve a KYC request", "Reject a KYC request"). (Microsoft Style Guide)
9. **Explain every term the first time it appears**, and again in the glossary. Avoid developer words in the body of the guide.
10. **Warn before the risky step, not after.** Put a **Careful** box before any action that moves money, locks an account or cannot be undone.
11. **Do not rely on colour alone.** Every callout starts with a word (Tip, Careful, Not available yet). Every badge in a table is described in words.
12. **Show, then tell.** Add a screenshot for each key step with numbered red badges that match the step numbers. Give every image alt text and a caption ("Figure N: ...").
13. **Do not use ">" menu paths such as Menu > Go To > Folders.** Screen readers may read them wrongly. Write "Click **Fraud Monitor** in the left menu." (Microsoft Style Guide)
14. **Say plainly what does not exist.** If a button is greyed out or a page says "API coming", describe it in a **Not available yet** box. Never describe a feature the product lacks.
15. **Keep each chapter short.** At most two printed pages per chapter; link to the glossary instead of repeating a definition.

## Sources

- Microsoft Writing Style Guide, "Writing step-by-step instructions": https://learn.microsoft.com/en-us/style-guide/procedures-instructions/writing-step-by-step-instructions
- Microsoft Writing Style Guide, "Procedures and instructions checklist": https://learn.microsoft.com/en-us/style-guide/checklists/procedures-and-instructions-checklist
- Google developer documentation style guide, "Procedures": https://developers.google.com/style/procedures
- Federal Plain Language Guidelines (plainlanguage.gov), "Write for your audience" and "Write short sentences": https://github.com/GSA/plainlanguage.gov/blob/main/_pages/guidelines/audience/index.md and https://digital.gov/guides/plain-language/principles

Limit of this research: the web-search tool returned summaries, not full page text. Rules 4, 5, 7, 8 and 13 come from the Microsoft and Google summaries; rules 2, 3 and 1 from the plain-language guidelines. Rules 9 to 12, 14 and 15 are HPay house rules chosen for this audience. No published "agent skill" for user guides was adopted: third-party skills turned up in search results (for example copies of the Google style guide) but were not downloaded or trusted.
