import { Ionicons } from '@expo/vector-icons';
import { Tabs } from 'expo-router';
import { useEffect } from 'react';
import { StyleSheet } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withSpring } from 'react-native-reanimated';

import { color } from '@/src/ui';

function TabIcon({ name, focused }: { name: keyof typeof Ionicons.glyphMap; focused: boolean }) {
  const scale = useSharedValue(1);

  useEffect(() => {
    scale.value = focused ? withSpring(1.15, { damping: 12, stiffness: 150 }) : withSpring(1);
  }, [focused, scale]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
  }));

  const iconName = focused ? name : (`${String(name)}-outline` as keyof typeof Ionicons.glyphMap);

  return (
    <Animated.View style={[styles.iconContainer, animatedStyle]}>
      <Ionicons
        name={iconName}
        size={22}
        color={focused ? color.brand[600] : color.neutral[400]}
      />
    </Animated.View>
  );
}

export default function MainTabsLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: color.brand[600],
        tabBarInactiveTintColor: color.neutral[500],
        tabBarStyle: styles.tabBar,
        tabBarLabelStyle: styles.tabBarLabel,
      }}
    >
      <Tabs.Screen
        name="home"
        options={{
          title: 'Home',
          tabBarIcon: ({ focused }) => <TabIcon name="home" focused={focused} />,
        }}
      />
      <Tabs.Screen
        name="history"
        options={{
          title: 'History',
          tabBarIcon: ({ focused }) => <TabIcon name="time" focused={focused} />,
        }}
      />
      <Tabs.Screen
        name="qr"
        options={{
          title: 'QR',
          tabBarIcon: ({ focused }) => <TabIcon name="qr-code" focused={focused} />,
        }}
      />
      <Tabs.Screen
        name="account"
        options={{
          title: 'Account',
          tabBarIcon: ({ focused }) => <TabIcon name="person" focused={focused} />,
        }}
      />
      {/* Flow stacks & detail live inside the tab navigator but never appear in the bar. */}
      <Tabs.Screen name="topup" options={{ href: null, tabBarStyle: { display: 'none' } }} />
      <Tabs.Screen name="transfer" options={{ href: null, tabBarStyle: { display: 'none' } }} />
      <Tabs.Screen name="requests" options={{ href: null, tabBarStyle: { display: 'none' } }} />
      <Tabs.Screen name="security" options={{ href: null, tabBarStyle: { display: 'none' } }} />
      <Tabs.Screen name="org" options={{ href: null, tabBarStyle: { display: 'none' } }} />
      <Tabs.Screen name="txn-detail" options={{ href: null }} />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  iconContainer: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  tabBar: {
    height: 60,
    paddingBottom: 8,
    paddingTop: 8,
    borderTopWidth: 1,
    borderTopColor: color.neutral[200],
    backgroundColor: '#fff',
    elevation: 8,
    shadowColor: color.neutral[900],
    shadowOffset: { width: 0, height: -2 },
    shadowOpacity: 0.05,
    shadowRadius: 3,
  },
  tabBarLabel: {
    fontSize: 11,
    fontWeight: '600',
  },
});

