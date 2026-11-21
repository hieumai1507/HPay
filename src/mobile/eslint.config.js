const expoConfig = require('eslint-config-expo/flat');

// C-01: money is bigint end to end. Number()/parseInt()/parseFloat() on anything named like
// an amount is a float bug waiting to happen. Use parseAmountStrict / parseAmountToBigInt
// from src/lib/money.ts. (src/lib/__tests__/no-float-money.test.ts enforces the same rule
// under jest.)
const MONEY_NAME = '/^(amount|balance|fee|total|target|min_?amount|max_?amount)/i';

module.exports = [
  ...expoConfig,
  {
    ignores: ['dist/*', 'src/api/schema.d.ts'],
  },
  {
    files: ['app/**/*.{ts,tsx}', 'src/**/*.{ts,tsx}'],
    ignores: ['**/__tests__/**'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: `CallExpression[callee.name=/^(Number|parseInt|parseFloat)$/] > Identifier.arguments[name=${MONEY_NAME}]`,
          message: 'Do not apply Number()/parseInt()/parseFloat() to money. Use BigInt helpers from src/lib/money.ts.',
        },
        {
          selector: `CallExpression[callee.name=/^(Number|parseInt|parseFloat)$/] > MemberExpression.arguments[property.name=${MONEY_NAME}]`,
          message: 'Do not apply Number()/parseInt()/parseFloat() to money. Use BigInt helpers from src/lib/money.ts.',
        },
        {
          selector: `CallExpression[callee.name=/^(Number|parseInt|parseFloat)$/] > CallExpression.arguments > MemberExpression.callee > Identifier.object[name=${MONEY_NAME}]`,
          message: 'Do not apply Number()/parseInt()/parseFloat() to money. Use BigInt helpers from src/lib/money.ts.',
        },
      ],
    },
  },
];