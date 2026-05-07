import { defineRule } from "@openbpl/sdk";

export default defineRule({
  name: "login-form",
  description: "Detects credential harvesting forms with password inputs",

  evaluate({ evidence, brand }) {
    const html = evidence.html.toLowerCase();

    const hasPassword = /<input[^>]+type=["']password["'][^>]*>/i.test(html);
    if (!hasPassword) return null;

    const hasEmail =
      /<input[^>]+type=["'](?:email|text)["'][^>]*name=["'](?:email|user|login|username)["'][^>]*>/i.test(
        html
      );

    let confidence = 0.5;
    let detail = "password input detected";

    if (hasEmail) {
      confidence = 0.8;
      detail = "login form with email/username + password inputs";
    }

    // Check if form posts to external domain
    const formAction = /<form[^>]+action=["']([^"']+)["'][^>]*>/i.exec(
      evidence.html
    );
    if (formAction && formAction[1]) {
      const action = formAction[1];
      if (action && !action.includes(evidence.domain)) {
        confidence = 0.95;
        detail += `; form posts to external domain: ${action}`;
      }
    }

    return { name: "login-form", confidence, detail };
  },
});
