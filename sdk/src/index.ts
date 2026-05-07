/**
 * Evidence collected from a captured page.
 */
export interface Evidence {
  /** The domain that was captured (e.g. "paypal-login.xyz") */
  domain: string;
  /** Full HTML content of the page */
  html: string;
  /** Page title extracted from HTML */
  title: string;
  /** Path to the screenshot PNG on disk */
  screenshotPath: string;
  /** Base64-encoded screenshot PNG */
  screenshot: string;
}

/**
 * Brand information from the project config.
 */
export interface Brand {
  name: string;
  website: string;
  description: string;
  industry: string;
  keywords: {
    included: string[];
    excluded: string[];
  };
  images: string[];
  colors: string[];
  urls: {
    domains: string[];
    socialMedia: string[];
    appStores: string[];
    browserExtensions: string[];
    blogs: string[];
  };
}

/**
 * Context passed to every rule evaluation.
 */
export interface RuleContext {
  evidence: Evidence;
  brand: Brand;
}

/**
 * A label emitted by a rule when it detects something.
 */
export interface Label {
  /** Name of this detection (e.g. "favicon-match", "login-form") */
  name: string;
  /** Confidence score from 0.0 to 1.0 */
  confidence: number;
  /** Human-readable detail about what was detected */
  detail: string;
}

/**
 * A rule definition.
 */
export interface RuleDefinition {
  /** Unique name for this rule */
  name: string;
  /** Human-readable description */
  description?: string;
  /** The evaluation function */
  evaluate: (ctx: RuleContext) => Label[] | Label | null | Promise<Label[] | Label | null>;
}

/**
 * Define a rule. This is the main entry point for rule authors.
 */
export function defineRule(definition: RuleDefinition): RuleDefinition {
  return definition;
}
