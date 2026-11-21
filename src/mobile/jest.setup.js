// Mock expo vector icons to prevent asynchronous font loading state updates during tests
jest.mock('@expo/vector-icons', () => {
  const React = require('react');
  const { View } = require('react-native');
  const makeIcon = (name) => {
    const Icon = React.forwardRef(function Icon(props, ref) {
      return <View ref={ref} {...props} />;
    });
    Icon.displayName = name;
    return Icon;
  };
  return {
    Ionicons: makeIcon('Ionicons'),
    Feather: makeIcon('Feather'),
  };
});