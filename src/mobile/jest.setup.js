// Mock expo vector icons to prevent asynchronous font loading state updates during tests
jest.mock('@expo/vector-icons', () => {
  const React = require('react');
  const { View } = require('react-native');
  return {
    Ionicons: React.forwardRef((props, ref) => <View ref={ref} {...props} />),
    Feather: React.forwardRef((props, ref) => <View ref={ref} {...props} />),
  };
});
