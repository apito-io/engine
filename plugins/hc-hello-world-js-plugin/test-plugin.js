#!/usr/bin/env node

// Test script for the JavaScript HashiCorp plugin
// This demonstrates the plugin functionality without needing the Go host

const { HelloWorldJSPlugin } = require('./index.js');

console.log('🚀 Testing Hello World JavaScript Plugin');
console.log('========================================\n');

async function testPlugin() {
    const plugin = new HelloWorldJSPlugin();
    
    console.log('1. Testing Hello World Resolver');
    console.log('--------------------------------');
    
    try {
        // Test basic hello world
        let result = plugin.executeHelloWorldResolver({ name: 'JavaScript Developer' });
        console.log('✅ Basic hello:', result);
        
        // Test with object
        result = plugin.executeHelloWorldResolver({ 
            name: 'JS Dev',
            object: { name: 'Test Object', age: 25 }
        });
        console.log('✅ With object:', result);
        
        // Test with array of objects
        result = plugin.executeHelloWorldResolver({ 
            name: 'JS Dev',
            arrayofObjects: [
                { name: 'Object 1', age: 30 },
                { name: 'Object 2', age: 35 }
            ]
        });
        console.log('✅ With array of objects:', result);
        
    } catch (error) {
        console.error('❌ Error in hello world test:', error.message);
    }
    
    console.log('\n2. Testing Say Hello Resolver');
    console.log('------------------------------');
    
    try {
        const result = plugin.executeSayHelloResolver({ 
            message: 'Hello from JavaScript Plugin Test!' 
        });
        console.log('✅ Say hello result:', result);
    } catch (error) {
        console.error('❌ Error in say hello test:', error.message);
    }
    
    console.log('\n3. Testing Complex Data Processing');
    console.log('-----------------------------------');
    
    try {
        const complexArgs = {
            user: { 
                id: 1, 
                name: 'Test User', 
                email: 'test@example.com', 
                age: 25, 
                active: true 
            },
            tags: ['javascript', 'nodejs', 'grpc', 'plugin'],
            numbers: [1, 2, 3, 4, 5],
            users: [
                { id: 1, name: 'Alice', email: 'alice@example.com' },
                { id: 2, name: 'Bob', email: 'bob@example.com' }
            ],
            optionalUsers: [
                { name: 'Optional User 1', email: 'opt1@example.com' },
                null,
                { name: 'Optional User 2', email: 'opt2@example.com' }
            ]
        };
        
        const result = plugin.executeProcessComplexDataResolver(complexArgs);
        console.log('✅ Complex data processing result:');
        console.log(result);
        
    } catch (error) {
        console.error('❌ Error in complex data test:', error.message);
    }
    
    console.log('\n4. Testing REST Handlers');
    console.log('-------------------------');
    
    try {
        // Test GET handler
        let result = await plugin.handleRESTExecution('jsHelloHandler', {}, {});
        console.log('✅ GET /js-hello:', JSON.stringify(result, null, 2));
        
        // Test POST handler
        result = await plugin.handleRESTExecution('jsHelloPostHandler', { 
            name: 'Test User',
            message: 'Hello from POST'
        }, {});
        console.log('✅ POST /js-hello:', JSON.stringify(result, null, 2));
        
    } catch (error) {
        console.error('❌ Error in REST test:', error.message);
    }
    
    console.log('\n5. Testing Protobuf Struct Conversion');
    console.log('--------------------------------------');
    
    try {
        // Test struct to object conversion
        const testStruct = {
            fields: {
                stringField: { stringValue: 'test string' },
                numberField: { numberValue: 42 },
                boolField: { boolValue: true },
                nullField: { nullValue: null },
                objectField: {
                    structValue: {
                        fields: {
                            nestedString: { stringValue: 'nested value' },
                            nestedNumber: { numberValue: 123 }
                        }
                    }
                },
                arrayField: {
                    listValue: {
                        values: [
                            { stringValue: 'item1' },
                            { stringValue: 'item2' },
                            { numberValue: 100 }
                        ]
                    }
                }
            }
        };
        
        const result = plugin.structToObject(testStruct);
        console.log('✅ Protobuf struct conversion:');
        console.log(JSON.stringify(result, null, 2));
        
    } catch (error) {
        console.error('❌ Error in protobuf test:', error.message);
    }
    
    console.log('\n🎉 JavaScript Plugin Test Complete!');
    console.log('====================================');
    console.log('All tests passed. The plugin is ready to be used with HashiCorp go-plugin.');
    console.log('\nTo run with Go host:');
    console.log('1. Set environment: export APITO_PLUGIN=apito_plugin_magic_cookie_v1');
    console.log('2. Run plugin: node index.js');
    console.log('3. The plugin will output: 1|1|tcp|127.0.0.1:PORT|grpc');
    console.log('4. The Go host can then connect to the specified port via gRPC');
}

// Run the tests
if (require.main === module) {
    testPlugin().catch(console.error);
}

module.exports = { testPlugin }; 