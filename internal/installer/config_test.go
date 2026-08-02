package installer

var testDrupalConfig = InstallationConfig{
	CommandName:          "install",
	Type:                 "drupal",
	ProductName:          "Drupal",
	MinimumDrupalVersion: 8,
	MaximumDrupalVersion: 12,
}

var testExtensionConfig = InstallationConfig{
	CommandName:          "store",
	Type:                 "store",
	ProductName:          "Drupal Store",
	MinimumDrupalVersion: 10,
	MaximumDrupalVersion: 11,
	ComposerPackages:     []string{"drupal/store:^1.0"},
	EnabledModules:       []string{"store", "store_cart"},
}
